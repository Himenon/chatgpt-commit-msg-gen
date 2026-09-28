# Architecture

## 生成フロー

```text
git commit
  └─ Lefthook: prepare-commit-msg
       └─ chatgpt-commit-msg-gen
            ├─ merge / -m 指定なら終了
            ├─ git diff --cached --no-color
            ├─ Codex CLI（read-only・ephemeral）
            ├─ 失敗時: OpenAI Responses API
            └─ Conventional Commits形式を検証してCOMMIT_EDITMSGへ挿入
```

`COMMIT_GENERATOR=codex` または `api` で生成元を固定できます。既定の `auto` はCodex CLIを優先し、Codex CLIが失敗した場合に `OPENAI_API_KEY` があればAPIへフォールバックします。Codex CLIが成功すればAPIキーは不要です。Codex CLIは `codex exec -` にプロンプトをstdinで渡し、`--sandbox read-only` と `--ephemeral` で実行します。`--output-last-message` で最終応答を受け取るため、コミットメッセージ生成以外のリポジトリ変更やセッション保存を避けます。

OpenAI APIは `POST /v1/responses` を直接呼び出します。既定モデルは `gpt-6-luna` で、リクエストは `store: false`、`text.verbosity: low` とし、出力トークン上限は512です。Responses APIの `output` 配列全体からテキストを集約します。外部Goパッケージには依存しません。

## 安全性と失敗時の挙動

- staged diffは最大50,000バイトに制限する
- CLI/APIは30秒でタイムアウトする
- 生成結果が `type(scope): subject` または `type: subject` でなければ書き込まない
- merge commitと `git commit -m` は変更しない
- 認証・ネットワーク・設定・書き込みの全エラーで終了コード0を維持し、コミットを止めない
- 既存の `COMMIT_EDITMSG`（Gitのコメント等）は生成行の後に保持する

## 配布

`scripts/build.sh` がGoバイナリをmacOS/Linux × arm64/amd64向けに生成します。npmパッケージには4バイナリとOS/CPU判定ラッパーを含めます。GitHub Releaseにはプラットフォーム別バイナリを登録し、`scripts/install.sh` から直接導入できます。
