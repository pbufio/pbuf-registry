package cmd

import (
	"bufio"
	"fmt"
	"io"
	"strings"

	"github.com/pbufio/pbuf-registry/internal/cli/model"
	"github.com/spf13/cobra"
)

func CreateInitCmd() *cobra.Command {
	var domainFlag string

	cmd := &cobra.Command{
		Use:   "init [module_name] [registry_url]",
		Short: "Init",
		Long:  "Init is a command to initialize pbuf.yaml",
		Args:  cobra.RangeArgs(0, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			var moduleName string
			var registryURL string
			var modules []string
			var paths []string

			if len(args) > 0 {
				moduleName = args[0]
				if len(args) > 1 {
					registryURL = args[1]
				} else if domainFlag != "" {
					registryURL = domainFlag
				} else {
					registryURL = "pbuf.cloud"
				}
			} else {
				scanner := bufio.NewScanner(cmd.InOrStdin())
				out := cmd.OutOrStdout()
				var err error
				if domainFlag != "" {
					registryURL = domainFlag
				} else {
					registryURL, err = promptLine(scanner, out, "Please provide registry URL", "pbuf.cloud")
					if err != nil {
						return fmt.Errorf("init aborted: %w", err)
					}
				}
				moduleName, err = promptLine(scanner, out, "Please provide module name", "domain/module-name")
				if err != nil {
					return fmt.Errorf("init aborted: %w", err)
				}
				modules, err = promptMultiLine(scanner, out, "Please provide modules", "")
				if err != nil {
					return fmt.Errorf("init aborted: %w", err)
				}
				paths, err = promptMultiLine(scanner, out, "Please provide paths", "api")
				if err != nil {
					return fmt.Errorf("init aborted: %w", err)
				}
			}

			registryURL = sanitizeRegistryURL(registryURL)

			var pbufModules []*model.Module
			for _, module := range modules {
				if trimmed := strings.TrimSpace(module); trimmed != "" {
					pbufModules = append(pbufModules, &model.Module{
						Name: trimmed,
					})
				}
			}

			var pbufPaths []string
			for _, path := range paths {
				if trimmed := strings.TrimSpace(path); trimmed != "" {
					pbufPaths = append(pbufPaths, trimmed)
				}
			}
			if len(pbufPaths) == 0 {
				pbufPaths = append(pbufPaths, "api")
			}

			pbufYaml := &model.Config{
				Version: "v1",
				Name:    moduleName,
				Registry: model.Registry{
					Addr: registryURL,
				},
				Export: model.Export{
					Paths: pbufPaths,
				},
				Modules: pbufModules,
			}

			if err := pbufYaml.Save(); err != nil {
				return fmt.Errorf("failed to save pbuf.yaml: %w", err)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&domainFlag, "domain", "", "registry domain or address (e.g. pbuf.cloud or localhost:6777)")
	cmd.Flags().StringVar(&domainFlag, "registry", "", "registry domain or address (e.g. pbuf.cloud or localhost:6777)")

	return cmd
}

func sanitizeRegistryURL(raw string) string {
	s := strings.TrimSpace(raw)
	s = strings.TrimPrefix(s, "https://")
	s = strings.TrimPrefix(s, "http://")
	s = strings.TrimRight(s, "/")
	return s
}

func promptLine(scanner *bufio.Scanner, out io.Writer, question, placeholder string) (string, error) {
	if _, err := fmt.Fprintf(out, "%s [%s]: ", question, placeholder); err != nil {
		return "", err
	}
	if !scanner.Scan() {
		if err := scanner.Err(); err != nil {
			return "", err
		}
		return "", io.ErrUnexpectedEOF
	}
	val := strings.TrimSpace(scanner.Text())
	if val == "" {
		return placeholder, nil
	}
	return val, nil
}

func promptMultiLine(scanner *bufio.Scanner, out io.Writer, question, placeholder string) ([]string, error) {
	prompt := fmt.Sprintf("%s (one per line, empty line to finish): ", question)
	if placeholder != "" {
		prompt = fmt.Sprintf("%s (one per line, empty line to finish) [%s]: ", question, placeholder)
	}
	if _, err := fmt.Fprint(out, prompt); err != nil {
		return nil, err
	}
	var values []string
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			return values, nil
		}
		for _, item := range strings.Split(line, ",") {
			if trimmed := strings.TrimSpace(item); trimmed != "" {
				values = append(values, trimmed)
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return nil, io.ErrUnexpectedEOF
}
