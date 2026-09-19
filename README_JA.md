# HCAI

<img src="frontend/public/hcai/hcai-logo.svg" alt="HCAI" width="200" />

HCAI は WangYa-Technology が管理する AI API ゲートウェイです。Sub2API を基盤とし、独自のホームページ、料金表示、CC Switch 連携、決済・紹介報酬、多拠点運用機能を維持しています。

詳しい導入方法、開発手順、設定と制約は [メイン README（中国語）](README.md) を参照してください。このページは概要であり、完全な日本語訳ではありません。

## HCAI の主な変更

- ブランド専用ホームページ、テーマ切り替え、読み込み表示。
- モデル料金一覧の人民元・米ドル表示と、チャネル設定価格を優先する表示処理。
- プラグイン管理設定の永続化、外部リンクと埋め込み時の認証情報保護。
- CC Switch のデスクトップ OAuth コールバックと利用量表示。
- 引換コードの販売価格、紹介報酬、Alipay サンドボックス。
- 企業微信通知、ノード監視、OAuth 更新リースと分散タスク調整。

料金一覧の表示優先順位は、実際の課金優先順位を変更するものではありません。

## 配布と注意事項

[本リポジトリの Releases](https://github.com/WangYa-Technology/sub2api/releases) から、`-hcai` が付いたバージョンを選択してください。現在のリリースワークフローは Linux amd64 向けです。更新前にチェックサム、ビルド情報、データベースと設定のバックアップを確認してください。

一部のインストールスクリプトと compose は上流のリポジトリや `weishaw/sub2api:latest` を参照しています。そのまま実行しても HCAI が導入される保証はありません。HCAI 管理画面にはオンライン更新・ロールバック・再起動 API はありません。

## 資料とライセンス

- [HCAI カスタマイズ一覧（中国語）](docs/hcai-dev/HCAI自定义功能清单.md)
- [差分とコミット一覧（中国語）](docs/hcai-dev/HCAI全量差异与提交附录.md)
- [Issue](https://github.com/WangYa-Technology/sub2api/issues)
- [LICENSE](LICENSE)（GNU LGPL v3）

基盤となる [Sub2API](https://github.com/Wei-Shaw/sub2api) と貢献者に感謝します。元の著作権表示および第三者コンポーネントのライセンス義務は引き続き適用されます。サービス提供者の利用規約と各地域の法令を確認し、秘密情報を公開しないでください。
