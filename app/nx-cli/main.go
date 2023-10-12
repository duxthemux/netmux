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
	"regexp"
	"slices"
	"strings"

	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/urfave/cli/v2"

	"github.com/duxthemux/netmux/app/nx-cli/installer"
)

type Endpoint struct {
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
}

type ListRow struct {
	Type   string
	Name   string
	Parent string
	Desc   string
	Status string
}

func (l *ListRow) String() string {
	if l.Type == "SVC" {
		return fmt.Sprintf("%s.%s", l.Parent, l.Name)
	}

	return l.Name
}

type ListOutput struct {
	Endpoints []Endpoint `json:"endpoints"`
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

func list(ctx context.Context, cli http.Client, filter string) error {

	responseBytes, err := httpGet(ctx, cli, "https://nx/api/v1/services/")
	if err != nil {
		return err
	}

	out := ListOutput{}
	if err = json.Unmarshal(responseBytes, &out); err != nil {
		return fmt.Errorf("error unmarshalling status: %w", err)
	}

	var rx *regexp.Regexp

	if filter != "" {
		filter = strings.ReplaceAll(filter, "+", ".*")
		rx, err = regexp.Compile(filter)
		if err != nil {
			return err
		}
	}

	rows := make([]ListRow, 0)
	for _, endpoint := range out.Endpoints {
		row := ListRow{
			Type:   "EP",
			Name:   endpoint.Name,
			Parent: "--",
			Desc: fmt.Sprintf(
				"%s %s.%s:%s",
				endpoint.Kubernetes.Context,
				endpoint.Kubernetes.Namespace,
				endpoint.Kubernetes.Endpoint,
				endpoint.Kubernetes.Port),
			Status: endpoint.Status,
		}

		if rx != nil {
			if rx.MatchString(row.String()) {
				rows = append(rows, row)
			}
		} else {
			rows = append(rows, row)
		}

		for _, svc := range endpoint.Bridges {

			row := ListRow{
				Type:   "SVC",
				Name:   svc.Name,
				Parent: endpoint.Name,
				Desc: fmt.Sprintf(
					"%s %s: %s:%s => %s:%s",
					svc.Name,
					svc.Direction,
					svc.LocalAddr,
					svc.LocalPort,
					svc.ContainerAddr,
					svc.ContainerPort),
				Status: svc.Status,
			}

			if rx != nil {
				if rx.MatchString(row.String()) {
					rows = append(rows, row)
				}
			} else {
				rows = append(rows, row)
			}

		}
	}

	slices.SortFunc(rows, func(a, b ListRow) int {
		if a.Parent == b.Parent {
			return strings.Compare(a.Name, b.Name)
		}

		return strings.Compare(a.Parent, b.Parent)
	})

	tbWriter := table.NewWriter()
	tbWriter.SetOutputMirror(os.Stdout)

	defer tbWriter.Render()

	tbWriter.AppendHeader(table.Row{"#", "K", "Name", "Parent", "Description", "Status"})

	for i, row := range rows {
		tbWriter.AppendRow(table.Row{
			fmt.Sprintf("%03d", i),
			row.Type,
			row.Name,
			row.Parent,
			row.Desc,
			row.Status,
		})

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

func reload(ctx context.Context, cli http.Client) error {
	_, err := httpGet(ctx, cli, "https://nx/api/v1/misc/reload")

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

	listFilter := ""

	app := &cli.App{
		Name:  "nx",
		Usage: "netmux command line client",
		Commands: []*cli.Command{
			{
				Name:  "install",
				Usage: "Install nx-daemon in your machine (call w root/admin) - Only for MAC ATM",
				Action: func(_ *cli.Context) error {
					myInstaller := installer.New()
					return myInstaller.Install()
				},
			},
			{
				Name:  "uninstall",
				Usage: "Uninstall nx-daemon in your machine (call w root/admin)  - Only for MAC ATM",
				Action: func(_ *cli.Context) error {
					myInstaller := installer.New()
					return myInstaller.Uninstall()
				},
			},
			{
				Name:    "list",
				Aliases: []string{"ls", "l"},
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:        "filter",
						Category:    "filter",
						DefaultText: "",
						FilePath:    "",
						Usage:       "",
						Required:    false,
						Hidden:      false,
						HasBeenSet:  false,
						Value:       "",
						Destination: &listFilter,
						Aliases:     nil,
						EnvVars:     nil,
						TakesFile:   false,
						Action:      nil,
					},
				},
				Usage: "Lists known info",
				Action: func(cCtx *cli.Context) error {
					return list(ctx, httpCli, listFilter)
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
				Name:    "reload",
				Aliases: []string{},
				Usage:   "Reload the daemon config",
				Action: func(cCtx *cli.Context) error {
					return reload(ctx, httpCli)
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
