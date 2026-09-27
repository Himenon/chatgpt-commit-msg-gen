# chatgpt-commit-msg-gen

[Codex CLI](https://developers.openai.com/codex/cli/) または [OpenAI API](https://developers.openai.com/api/) で staged diff を分析し、Lefthook の `prepare-commit-msg` から Conventional Commits 形式のコミットメッセージを生成するCLIです。

既定ではCodex CLIを使い、利用できない場合や生成に失敗した場合はOpenAI Responses APIへフォールバックします。どちらの生成にも失敗した場合でも `git commit` は中断しません。

## セットアップ

```sh
# npm経由
pnpm add -g @himenon/chatgpt-commit-msg-gen

# またはGitHub Releaseから（Node.js不要）
curl -fsSL https://raw.githubusercontent.com/Himenon/chatgpt-commit-msg-gen/main/scripts/install.sh | sh

chatgpt-commit-msg-gen --version
```

Lefthookと、少なくとも一方の認証手段を準備します。

```sh
brew install lefthook

# 推奨: Codex CLIへログイン
codex login

# APIフォールバックも使う場合
export OPENAI_API_KEY="sk-..."
```

利用先リポジトリの `lefthook.yml` に設定し、`lefthook install` を実行します。

```yaml
prepare-commit-msg:
  jobs:
    - name: auto-commit-message
      run: chatgpt-commit-msg-gen {1} {2}
```

あとは通常どおり、メッセージを指定せずコミットします。

```sh
git add <files>
git commit
```

`git commit -m`、merge commit、staged diffがない場合は生成をスキップします。

## 設定

| 環境変数 | 既定値 | 説明 |
| --- | --- | --- |
| `COMMIT_GENERATOR` | `auto` | `auto`（Codex→API）、`codex`、`api` |
| `COMMIT_LANGUAGE` | `ja` | `ja` または `en`。`COMMIT_PROMPT`指定時は無視 |
| `COMMIT_PROMPT` | 内蔵プロンプト | 生成ルールを完全に上書き |
| `CODEX_MODEL` | Codex設定値 | Codex CLIで使うモデル |
| `OPENAI_API_KEY` | — | Responses APIの認証キー |
| `OPENAI_MODEL` | `gpt-6-luna` | APIで使うモデル |
| `OPENAI_MAX_OUTPUT_TOKENS` | `512` | APIの最大出力トークン数 |
| `OPENAI_BASE_URL` | `https://api.openai.com` | OpenAI互換エンドポイントのベースURL |

例:

```yaml
      env:
        COMMIT_GENERATOR: auto
        COMMIT_LANGUAGE: en
        CODEX_MODEL: gpt-6-sol
        OPENAI_MODEL: gpt-6-luna
```

プロジェクトの `lefthook.yml` を変更したくない場合は、`lefthook-local.yml` を `.gitignore` に追加して同じ設定を記述できます。

## 開発

```sh
pnpm test
pnpm run build
```

対応プラットフォームは macOS/Linux の arm64/amd64 です。構成の詳細は [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md) を参照してください。

