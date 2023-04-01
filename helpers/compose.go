package helpers

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/compose-spec/compose-go/types"
	"github.com/containerd/console"
	"github.com/docker/compose-cli/api/client"
	"github.com/docker/compose-cli/api/containers"
	"github.com/docker/compose-cli/cli/mobycli"
	"github.com/docker/compose-cli/utils/formatter"
	format "github.com/docker/compose/v2/cmd/formatter"

	"github.com/docker/compose/v2/pkg/api"
	"github.com/pkg/errors"
)

func NewComposeClient(ctx context.Context) (*client.Client, error) {
	return client.New(ctx)
}

func GetVersion(ctx context.Context, c *client.Client) (string, error) {
	versionResult, err := mobycli.ExecSilent(ctx)
	if versionResult == nil {
		return "", err
	}
	return string(versionResult), err
}

func Up(ctx context.Context, c client.Client, projectName, composeFilePath string, detach bool) error {
	if projectName == "" {
		projectName = "meshery"
	}
	project := types.Project{Name: projectName, WorkingDir: filepath.Dir(composeFilePath), ComposeFiles: []string{composeFilePath}}

	var logConsumer api.LogConsumer
	if detach {
		_, pipeWriter := io.Pipe()
		logConsumer = format.NewLogConsumer(ctx, pipeWriter, false, false)
	}
	return c.ComposeService().Up(ctx, &project, api.UpOptions{Start: api.StartOptions{Attach: logConsumer}})
}

func Rm(ctx context.Context, c client.Client, projectName, composeFilePath string, force bool) error {
	if projectName == "" {
		projectName = "meshery"
	}
	project := types.Project{Name: projectName, WorkingDir: filepath.Dir(composeFilePath), ComposeFiles: []string{composeFilePath}}

	return c.ComposeService().Remove(ctx, &project, api.RemoveOptions{Force: force})
}

func Stop(ctx context.Context, c client.Client, projectName, composeFilePath string, force bool) error {
	if projectName == "" {
		projectName = "meshery"
	}
	project := types.Project{Name: projectName, WorkingDir: filepath.Dir(composeFilePath), ComposeFiles: []string{composeFilePath}}

	return c.ComposeService().Stop(ctx, &project, api.StopOptions{})
}

func Ps(ctx context.Context, c *client.Client, all, quiet bool, formatOpt string) error {
	containerList, err := c.ContainerService().List(ctx, all)
	if err != nil {
		return errors.Wrap(err, "fetch containers")
	}

	if quiet {
		for _, c := range containerList {
			fmt.Println(c.ID)
		}
		return nil
	}

	view := viewFromContainerList(containerList)
	return format.Print(view, formatOpt, os.Stdout, func(w io.Writer) {
		for _, c := range view {
			_, _ = fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n", c.ID, c.Image, c.Command, c.Status,
				strings.Join(c.Ports, ", "))
		}
	}, "CONTAINER ID", "IMAGE", "COMMAND", "STATUS", "PORTS")
}

type containerView struct {
	ID      string
	Image   string
	Status  string
	Command string
	Ports   []string
}

func fqdn(container containers.Container) string {
	fqdn := ""
	if container.Config != nil {
		fqdn = container.Config.FQDN
	}
	return fqdn
}

func viewFromContainerList(containerList []containers.Container) []containerView {
	retList := make([]containerView, len(containerList))
	for i, c := range containerList {
		retList[i] = containerView{
			ID:      c.ID,
			Image:   c.Image,
			Status:  c.Status,
			Command: c.Command,
			Ports:   formatter.PortsToStrings(c.Ports, fqdn(c)),
		}
	}
	return retList
}

func Pull(ctx context.Context, c *client.Client, composeFilePath, projectName string) error {
	if projectName == "" {
		projectName = "meshery"
	}
	project := types.Project{Name: projectName, WorkingDir: filepath.Dir(composeFilePath), ComposeFiles: []string{composeFilePath}}

	return c.ComposeService().Pull(ctx, &project, api.PullOptions{})
}

func GetLogs(ctx context.Context, c *client.Client, composeFilePath, tail string, follow bool) error {
	req := containers.LogsRequest{
		Follow: follow,
		Tail:   tail,
	}

	var con io.Writer = os.Stdout
	if cff, err := console.ConsoleFromFile(os.Stdout); err == nil {
		size, err := cff.Size()
		if err != nil {
			return err
		}
		req.Width = int(size.Width)
		con = cff
	}

	req.Writer = con

	return c.ContainerService().Logs(ctx, composeFilePath, req)
}
