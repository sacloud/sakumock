package securitycontrol

// ruleCatalog lists the evaluation rules every project has, in the order of
// the spec's EvaluationRuleID enum. The descriptions are the details column of
// the rule table in the Security Control manual
// (https://manual.sakura.ad.jp/cloud/controlpanel/eventlog/security-control.html).
var ruleCatalog = []struct {
	ID          string
	Description string
}{
	{"server-no-public-ip", "サーバーにパブリックIPアドレスが付与されていないかを確認します。"},
	{"disk-encryption-enabled", "ディスクの暗号化機能が有効になっているかを確認します。"},
	{"dba-encryption-enabled", "データベースアプライアンスのディスク暗号化機能が有効になっているかを確認します。"},
	{"dba-no-public-ip", "データベースアプライアンスにパブリックIPアドレスが付与されていないかを確認します。レプリケーションインスタンス(リードレプリカ)も含む全てのDBアプライアンスが対象となります。"},
	{"objectstorage-bucket-acl-changed", "オブジェクトストレージのバケットACLが変更されたことを検知します。"},
	{"objectstorage-bucket-encryption-enabled", "指定したサイトに存在するすべてのバケットについて、バケット単位の暗号化設定を確認します。"},
	{"addon-datalake-no-public-access", "データレイクストレージへのパブリックネットワークアクセス設定有効化を検知します。"},
	{"addon-dwh-no-public-access", "データウェアハウスストレージへのパブリックネットワークアクセス設定有効化を検知します。"},
	{"addon-threat-detection-enabled", "指定されたゾーン内にある対象プロジェクトが所有する全てのサーバで、Add-onの脅威検知および脆弱性検知サービスのエージェントがインストールされていることを確認します。"},
	{"elb-logging-enabled", "エンハンスドLBでsyslog転送機能が有効になっていることを確認します。"},
	{"iam-member-operation-detected", "イベントログ一覧で「現在選択中のプロジェクト」に記録される、会員による操作イベントを検知します。"},
	{"nosql-encryption-enabled", "暗号化が無効なNoSQLアプライアンスが1つ以上存在するとき、検出します。"},
	{"addon-threat-detections", "Add-on (Defender for Cloud)によるセキュリティインシデントアラート（脅威検知）が発生した際にセキュリティコントロール上に表示させるための評価ルールです。"},
	{"addon-vulnerability-detections", "Add-on (Defender for Cloud)による推奨事項（脆弱性）の検出が行われた際にセキュリティコントロール上に表示させるための評価ルールです。"},
}

// ruleDescription returns the catalog description of a rule ID.
func ruleDescription(id string) string {
	for _, r := range ruleCatalog {
		if r.ID == id {
			return r.Description
		}
	}
	return ""
}
