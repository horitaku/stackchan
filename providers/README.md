# providers

このディレクトリは provider の共有仕様を管理します。

- provider 契約（interface 仕様、リクエスト/レスポンス要件）
- adapter の互換性メモ
- 将来の共通 fixture と検証データ

実行コードは当面 `server/internal/providers` に集約し、
ランタイム責務の分離と依存関係の明確化を優先します。
