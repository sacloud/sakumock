// Command sakumock is the unified CLI for the SAKURA Cloud mock server suite.
// Each service is exposed as a subcommand (e.g. "sakumock simplemq") that
// accepts the same flags as the standalone sakumock-<service> binaries.
package main

import (
	"context"

	"github.com/alecthomas/kong"
	"github.com/sacloud/sakumock"
	"github.com/sacloud/sakumock/addon"
	"github.com/sacloud/sakumock/apigw"
	"github.com/sacloud/sakumock/apprun"
	"github.com/sacloud/sakumock/apprundedicated"
	"github.com/sacloud/sakumock/cloudhsm"
	"github.com/sacloud/sakumock/core"
	"github.com/sacloud/sakumock/eventbus"
	"github.com/sacloud/sakumock/iam"
	"github.com/sacloud/sakumock/kms"
	"github.com/sacloud/sakumock/monitoringsuite"
	"github.com/sacloud/sakumock/objectstorage"
	"github.com/sacloud/sakumock/secretmanager"
	"github.com/sacloud/sakumock/seg"
	"github.com/sacloud/sakumock/simplemq"
	"github.com/sacloud/sakumock/simplenotification"
	"github.com/sacloud/sakumock/workflows"
)

type CLI struct {
	All                AllCmd                     `cmd:"" name:"all" help:"Run all mock services together in one process"`
	Env                EnvCmd                     `cmd:"" name:"env" help:"Print client environment variables (endpoints + dummy credentials) as a dotenv file and exit"`
	Inspect            InspectCmd                 `cmd:"" name:"inspect" help:"Query or drive a running sakumock's inspection endpoints"`
	Docs               DocsCmd                    `cmd:"" name:"docs" help:"Print the embedded documentation (index, a service README, or a section of one)"`
	Simplemq           simplemq.Command           `cmd:"" name:"simplemq" help:"SimpleMQ mock server"`
	Kms                kms.Command                `cmd:"" name:"kms" help:"KMS mock server"`
	Secretmanager      secretmanager.Command      `cmd:"" name:"secretmanager" help:"SecretManager mock server"`
	Simplenotification simplenotification.Command `cmd:"" name:"simplenotification" help:"Simple Notification mock server"`
	Monitoringsuite    monitoringsuite.Command    `cmd:"" name:"monitoringsuite" help:"Monitoring Suite mock server"`
	Eventbus           eventbus.Command           `cmd:"" name:"eventbus" help:"EventBus mock server"`
	Objectstorage      objectstorage.Command      `cmd:"" name:"objectstorage" help:"Object Storage mock server"`
	Iam                iam.Command                `cmd:"" name:"iam" help:"IAM mock server"`
	Apprun             apprun.Command             `cmd:"" name:"apprun" help:"AppRun mock server"`
	ApprunDedicated    apprundedicated.Command    `cmd:"" name:"apprun-dedicated" help:"AppRun Dedicated mock server"`
	Workflows          workflows.Command          `cmd:"" name:"workflows" help:"Workflows mock server"`
	Apigw              apigw.Command              `cmd:"" name:"apigw" help:"API Gateway mock server"`
	Cloudhsm           cloudhsm.Command           `cmd:"" name:"cloudhsm" help:"CloudHSM mock server"`
	Seg                seg.Command                `cmd:"" name:"seg" help:"Service Endpoint Gateway mock server"`
	Addon              addon.Command              `cmd:"" name:"addon" help:"Add-on mock server"`

	Version kong.VersionFlag `help:"Show version" short:"v"`
}

func main() {
	ctx, stop := core.NotifyContext(context.Background())
	defer stop()

	var cli CLI
	kctx := kong.Parse(&cli,
		kong.Name("sakumock"),
		kong.Description("Local mock server suite for SAKURA Cloud APIs. Run 'sakumock docs' for the embedded documentation index."),
		kong.UsageOnError(),
		kong.BindTo(ctx, (*context.Context)(nil)),
		kong.Vars{"version": sakumock.Version},
	)
	kctx.FatalIfErrorf(kctx.Run())
}
