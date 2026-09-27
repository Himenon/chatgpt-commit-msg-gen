package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

var version = "dev"
var apiHTTPClient = http.DefaultClient

const (
	defaultBaseURL         = "https://api.openai.com"
	defaultModel           = "gpt-6-luna"
	defaultMaxOutputTokens = 512
	maxDiffSize            = 50000
	requestTimeout         = 30 * time.Second
	defaultPromptJa        = `以下のgit diffを分析し、Conventional Commits形式のコミットメッセージを1行だけ生成してください。

形式: type(scope): subject
type: feat, fix, docs, style, refactor, test, chore, ci, perf

ルール:
- subjectは日本語で50文字以内にする
- 末尾に句読点を付けない
- scopeは変更対象のモジュール名（省略可）
- 抽象語や指示詞を避け、変更対象と処理内容を具体的に書く
- コミットメッセージ1行だけを出力し、説明やMarkdownを含めない`
	defaultPromptEn = `Analyze the git diff and output exactly one Conventional Commits message.

Format: type(scope): subject
Types: feat, fix, docs, style, refactor, test, chore, ci, perf

Rules:
- Write the subject in English and keep the full line within 72 characters
- Do not end with punctuation
- scope is optional and identifies the changed module
- Use concrete technical terms and avoid pronouns or vague wording
- Output one commit message line only, with no explanation or Markdown`
)

type responseRequest struct {
	Model           string `json:"model"`
	Input           string `json:"input"`
	MaxOutputTokens int    `json:"max_output_tokens"`
	Store           bool   `json:"store"`
	Text            struct {
		Verbosity string `json:"verbosity"`
	} `json:"text"`
}

type responseBody struct {
	Output []struct {
		Type    string `json:"type"`
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
	} `json:"output"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error"`
}

func main() {
	if len(os.Args) >= 2 && (os.Args[1] == "--version" || os.Args[1] == "-v") {
		fmt.Println(version)
		return
	}
	if len(os.Args) < 2 {
		logError("Usage: chatgpt-commit-msg-gen <commit-msg-file> [commit-source]")
		return
	}

	commitSource := ""
	if len(os.Args) >= 3 {
		commitSource = os.Args[2]
	}
	if commitSource == "merge" || commitSource == "message" || isMerging() {
		return
	}

	diff, err := getStagedDiff()
	if err != nil {
		logError("git diff の取得に失敗しました: %v", err)
		return
	}
	if diff == "" {
		return
	}
	if len(diff) > maxDiffSize {
		diff = diff[:maxDiffSize]
	}

	prompt := commitPrompt() + "\n\n---\n" + diff
	message, provider, err := generate(prompt)
	if err != nil {
		logError("コミットメッセージの生成に失敗しました: %v", err)
		return
	}
	message = cleanMessage(message)
	if !isConventionalCommit(message) {
		logError("生成結果がConventional Commits形式ではありません: %s", message)
		return
	}

	existing, _ := os.ReadFile(os.Args[1])
	if err := os.WriteFile(os.Args[1], []byte(message+"\n\n"+string(existing)), 0644); err != nil {
		logError("ファイルへの書き込みに失敗しました: %v", err)
		return
	}
	fmt.Fprintf(os.Stderr, "[chatgpt-commit-msg-gen] %sで生成しました: %s\n", provider, message)
}

func generate(prompt string) (string, string, error) {
	mode := strings.ToLower(os.Getenv("COMMIT_GENERATOR"))
	if mode == "" {
		mode = "auto"
	}
	if mode != "auto" && mode != "codex" && mode != "api" {
		return "", "", fmt.Errorf("COMMIT_GENERATOR は auto, codex, api のいずれかを指定してください")
	}
	var failures []string
	if mode == "auto" || mode == "codex" {
		msg, err := generateWithCodex(prompt)
		if err == nil {
			if cleaned, valid := validMessage(msg); valid {
				return cleaned, "Codex CLI", nil
			}
			err = fmt.Errorf("生成結果がConventional Commits形式ではありません")
		}
		failures = append(failures, "Codex CLI: "+err.Error())
		if mode == "codex" {
			return "", "", errors.New(strings.Join(failures, "; "))
		}
	}
	if mode == "auto" || mode == "api" {
		msg, err := generateWithAPI(prompt)
		if err == nil {
			if cleaned, valid := validMessage(msg); valid {
				return cleaned, "OpenAI API", nil
			}
			err = fmt.Errorf("生成結果がConventional Commits形式ではありません")
		}
		failures = append(failures, "OpenAI API: "+err.Error())
	}
	return "", "", errors.New(strings.Join(failures, "; "))
}

func validMessage(raw string) (string, bool) {
	message := cleanMessage(raw)
	return message, isConventionalCommit(message)
}

func generateWithCodex(prompt string) (string, error) {
	path, err := exec.LookPath("codex")
	if err != nil {
		return "", fmt.Errorf("codex コマンドが見つかりません")
	}
	tmp, err := os.CreateTemp("", "chatgpt-commit-msg-gen-*.txt")
	if err != nil {
		return "", err
	}
	name := tmp.Name()
	_ = tmp.Close()
	defer os.Remove(name)

	ctx, cancel := context.WithTimeout(context.Background(), requestTimeout)
	defer cancel()
	args := []string{"exec", "--ephemeral", "--sandbox", "read-only", "--color", "never", "--output-last-message", name}
	if model := os.Getenv("CODEX_MODEL"); model != "" {
		args = append(args, "--model", model)
	}
	args = append(args, "-")
	cmd := exec.CommandContext(ctx, path, args...)
	cmd.Stdin = strings.NewReader(prompt)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		if ctx.Err() != nil {
			return "", fmt.Errorf("%s", ctx.Err())
		}
		return "", fmt.Errorf("実行失敗: %v: %s", err, strings.TrimSpace(stderr.String()))
	}
	out, err := os.ReadFile(name)
	if err != nil {
		return "", fmt.Errorf("出力の読み取りに失敗: %w", err)
	}
	if strings.TrimSpace(string(out)) == "" {
		return "", fmt.Errorf("空の応答")
	}
	return string(out), nil
}

func generateWithAPI(prompt string) (string, error) {
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		return "", fmt.Errorf("OPENAI_API_KEY が未設定")
	}
	model := envOr("OPENAI_MODEL", defaultModel)
	maxTokens := defaultMaxOutputTokens
	if n, err := strconv.Atoi(os.Getenv("OPENAI_MAX_OUTPUT_TOKENS")); err == nil && n > 0 {
		maxTokens = n
	}
	body := responseRequest{Model: model, Input: prompt, MaxOutputTokens: maxTokens, Store: false}
	body.Text.Verbosity = "low"
	data, err := json.Marshal(body)
	if err != nil {
		return "", err
	}

	ctx, cancel := context.WithTimeout(context.Background(), requestTimeout)
	defer cancel()
	url := strings.TrimRight(envOr("OPENAI_BASE_URL", defaultBaseURL), "/") + "/v1/responses"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(data))
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")
	resp, err := apiHTTPClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("リクエスト失敗: %w", err)
	}
	defer resp.Body.Close()
	respData, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	var parsed responseBody
	if err := json.Unmarshal(respData, &parsed); err != nil {
		return "", fmt.Errorf("レスポンス解析失敗: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		if parsed.Error != nil {
			return "", fmt.Errorf("APIエラー (%d): %s", resp.StatusCode, parsed.Error.Message)
		}
		return "", fmt.Errorf("APIエラー (%d)", resp.StatusCode)
	}
	var output strings.Builder
	for _, item := range parsed.Output {
		for _, content := range item.Content {
			if content.Type == "output_text" && content.Text != "" {
				output.WriteString(content.Text)
			}
		}
	}
	if output.Len() > 0 {
		return output.String(), nil
	}
	return "", fmt.Errorf("レスポンスにテキストがありません")
}

func commitPrompt() string {
	if custom := os.Getenv("COMMIT_PROMPT"); custom != "" {
		return strings.TrimSpace(custom)
	}
	if os.Getenv("COMMIT_LANGUAGE") == "en" {
		return defaultPromptEn
	}
	return defaultPromptJa
}

func cleanMessage(raw string) string {
	var candidates []string
	for _, line := range strings.Split(raw, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.Contains(line, "`") {
			continue
		}
		if isConventionalCommit(line) {
			return line
		}
		candidates = append(candidates, line)
	}
	if len(candidates) > 0 {
		return candidates[0]
	}
	return ""
}

func isConventionalCommit(s string) bool {
	prefix, subject, ok := strings.Cut(strings.TrimSpace(s), ": ")
	if !ok || subject == "" {
		return false
	}
	typeName := prefix
	if i := strings.Index(prefix, "("); i >= 0 {
		if !strings.HasSuffix(prefix, ")") || i == 0 || i == len(prefix)-2 {
			return false
		}
		typeName = prefix[:i]
	}
	if typeName == "" {
		return false
	}
	for _, c := range typeName {
		if c < 'a' || c > 'z' {
			return false
		}
	}
	return true
}

func isMerging() bool {
	root, err := exec.Command("git", "rev-parse", "--show-toplevel").Output()
	if err != nil {
		return false
	}
	_, err = os.Stat(filepath.Join(strings.TrimSpace(string(root)), ".git", "MERGE_HEAD"))
	return err == nil
}

func getStagedDiff() (string, error) {
	out, err := exec.Command("git", "diff", "--cached", "--no-color").Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

func envOr(name, fallback string) string {
	if v := os.Getenv(name); v != "" {
		return v
	}
	return fallback
}
func logError(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "[chatgpt-commit-msg-gen] "+format+"\n", args...)
}
