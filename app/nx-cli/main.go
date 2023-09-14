package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"

	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/urfave/cli/v2"
)

type ListOutput struct {
	Endpoints []struct {
		Name       string `json:"name"`
		Endpoint   string `json:"endpoint"`
		Kubernetes struct {
			Config    string `json:"config"`
			Namespace string `json:"namespace"`
			Endpoint  string `json:"endpoint"`
			Context   string `json:"context"`
			Port      string `json:"port"`
		} `json:"kubernetes"`
		Status  string `json:"status"`
		Bridges []struct {
			Namespace     string `json:"namespace"`
			Name          string `json:"name"`
			LocalAddr     string `json:"localAddr"`
			LocalPort     string `json:"localPort"`
			ContainerAddr string `json:"containerAddr"`
			ContainerPort string `json:"containerPort"`
			Direction     string `json:"direction"`
			Family        string `json:"family"`
			Status        string `json:"status"`
		} `json:"bridges"`
	} `json:"endpoints"`
}

func httpGet(ctx context.Context, cli http.Client, url string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("error creating request: %w", err)
	}

	res, err := cli.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error calling http endpoint: %w", err)
	}

	defer func() {
		_ = res.Body.Close()
	}()

	buf := &bytes.Buffer{}

	_, err = io.Copy(buf, res.Body)
	if err != nil {
		return nil, fmt.Errorf("error reading response: %w", err)
	}

	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("error receiving response: %s - %s", res.Status, buf.String())
	}

	return buf.Bytes(), nil
}

func list(ctx context.Context, cli http.Client) error {
	responseBytes, err := httpGet(ctx, cli, "https://nx/api/v1/services/")
	if err != nil {
		return err
	}

	out := ListOutput{}
	if err = json.Unmarshal(responseBytes, &out); err != nil {
		return fmt.Errorf("error unmarshalling status: %w", err)
	}

	tbWriter := table.NewWriter()
	tbWriter.SetOutputMirror(os.Stdout)

	defer tbWriter.Render()

	tbWriter.AppendHeader(table.Row{"#", "Name", "Parent", "Description", "Status"})

	for _, endpoint := range out.Endpoints {
		tbWriter.AppendRow(table.Row{
			"EP",
			endpoint.Name,
			"",
			fmt.Sprintf(
				"%s %s.%s:%s",
				endpoint.Kubernetes.Context,
				endpoint.Kubernetes.Namespace,
				endpoint.Kubernetes.Endpoint,
				endpoint.Kubernetes.Port),
			endpoint.Status,
		})

		for _, svc := range endpoint.Bridges {
			tbWriter.AppendRow(
				table.Row{
					"SVC",
					svc.Name,
					endpoint.Name,
					fmt.Sprintf(
						"%s %s: %s:%s => %s:%s",
						svc.Name,
						svc.Direction,
						svc.LocalAddr,
						svc.LocalPort,
						svc.ContainerAddr,
						svc.ContainerPort),
					svc.Status,
				})
		}
	}

	return nil
}

func connect(ctx context.Context, cli http.Client, ctxName string) error {
	_, err := httpGet(ctx, cli, fmt.Sprintf("https://nx/api/v1/context/%s/connect", ctxName))

	return err
}

func disconnect(ctx context.Context, cli http.Client, ctxName string) error {
	_, err := httpGet(ctx, cli, fmt.Sprintf("https://nx/api/v1/context/%s/disconnect", ctxName))

	return err
}

func start(ctx context.Context, cli http.Client, ctxName string, svc string) error {
	_, err := httpGet(ctx, cli, fmt.Sprintf("https://nx/api/v1/services/%s/%s/start", ctxName, svc))

	return err
}

func stop(ctx context.Context, cli http.Client, ctxName string, svc string) error {
	_, err := httpGet(ctx, cli, fmt.Sprintf("https://nx/api/v1/services/%s/%s/stop", ctxName, svc))

	return err
}

func exit(ctx context.Context, cli http.Client) error {
	_, err := httpGet(ctx, cli, "https://nx/api/v1/misc/exit")

	return err
}

func cleanup(ctx context.Context, cli http.Client) error {
	_, err := httpGet(ctx, cli, "https://nx/api/v1/misc/cleanup")

	return err
}

//nolint:funlen
func main() {
	httpCli := http.Client{}
	ctx := context.Background()
	app := &cli.App{
		Name:  "nx",
		Usage: "netmux command line client",
		Commands: []*cli.Command{
			{
				Name:    "list",
				Aliases: []string{"ls", "l"},
				Usage:   "Lists known info",
				Action: func(cCtx *cli.Context) error {
					return list(ctx, httpCli)
				},
			},
			{
				Name:    "connect",
				Aliases: []string{"con", "c"},
				Usage:   "Connects to Endpoint [endpoint]",
				Action: func(cCtx *cli.Context) error {
					return connect(ctx, httpCli, cCtx.Args().Get(0))
				},
			},
			{
				Name:    "disconnect",
				Aliases: []string{"dis", "d"},
				Usage:   "Disconnects from Endpoint [endpoint]",
				Action: func(cCtx *cli.Context) error {
					return disconnect(ctx, httpCli, cCtx.Args().Get(0))
				},
			},
			{
				Name:    "start",
				Aliases: []string{"on", "+"},
				Usage:   "Starts service [endpoint] [svc]",
				Action: func(cCtx *cli.Context) error {
					return start(ctx, httpCli, cCtx.Args().Get(0), cCtx.Args().Get(1))
				},
			},
			{
				Name:    "stop",
				Aliases: []string{"off", "-"},
				Usage:   "Stops service [endpoint] [svc]",
				Action: func(cCtx *cli.Context) error {
					return stop(ctx, httpCli, cCtx.Args().Get(0), cCtx.Args().Get(1))
				},
			},
			{
				Name:    "exit",
				Aliases: []string{},
				Usage:   "Stops the daemon",
				Action: func(cCtx *cli.Context) error {
					return exit(ctx, httpCli)
				},
			},
			{
				Name:    "cleanup",
				Aliases: []string{},
				Usage:   "Cleans dns entries",
				Action: func(cCtx *cli.Context) error {
					return cleanup(ctx, httpCli)
				},
			},
		},

		Action: func(*cli.Context) error {
			return fmt.Errorf("unknown command")
		},
	}

	if err := app.Run(os.Args); err != nil {
		log.Fatal(err)
	}
}
