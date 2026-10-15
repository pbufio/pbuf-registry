package cmd

import (
	"fmt"
	"log"
	"strings"

	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/pbufio/pbuf-registry/internal/cli/model"
	"github.com/spf13/cobra"
)

type (
	errMsg error
)

type textModel struct {
	question  string
	textInput textinput.Model
	err       error
}

type textAreaModel struct {
	question string
	textArea textarea.Model
	err      error
}

func CreateInitCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "init",
		Short: "Init",
		Long:  "Init is a command to initialize pbuf.yaml",
		Args:  cobra.RangeArgs(0, 2),
		Run: func(cmd *cobra.Command, args []string) {
			var moduleName string
			var registryURL string
			var modules []string
			var paths []string

			if len(args) > 0 {
				moduleName = args[0]
				if len(args) > 1 {
					registryURL = args[0]
				} else {
					registryURL = "pbuf.cloud"
				}
			} else {
				registryURL = askQuestionAndGetAnswer("Please provide registry URL", "pbuf.cloud")
				moduleName = askQuestionAndGetAnswer("Please provide module name", "domain/module-name")
				modules = askMultiQuestionAndGetAnswer("Please provide modules", "domain/module-name")
				paths = askMultiQuestionAndGetAnswer("Please provide paths", "api")
			}

			var pbufModules []*model.Module
			for _, module := range modules {
				if module != "" {
					pbufModules = append(pbufModules, &model.Module{
						Name: module,
					})
				}
			}

			// backward compatibility
			if len(pbufModules) == 0 {
				pbufModules = append(pbufModules, &model.Module{
					Name: "pbufio/pbuf-registry",
				})
			}

			var pbufPaths []string
			for _, path := range paths {
				if path != "" {
					pbufPaths = append(pbufPaths, path)
				}
			}

			// backward compatibility
			if len(pbufPaths) == 0 {
				pbufPaths = append(pbufPaths, "api")
				pbufPaths = append(pbufPaths, "proto")
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

			err := pbufYaml.Save()
			if err != nil {
				log.Fatalf("failed to save pbuf.yaml: %v", err)
			}
		},
	}
}

func askQuestionAndGetAnswer(question, placeholder string) string {
	initInfo := createTextModel(question, placeholder)
	m, err := tea.NewProgram(initInfo).Run()
	if err != nil {
		log.Fatal(err)
	}
	return m.(textModel).textInput.Value()
}

// text model section

func createTextModel(question, placeholder string) textModel {
	ti := textinput.New()
	ti.Placeholder = placeholder
	ti.Focus()
	ti.CharLimit = 156
	ti.Width = 20

	return textModel{
		question:  question,
		textInput: ti,
		err:       nil,
	}
}

func (m textModel) Init() tea.Cmd {
	return textinput.Blink
}

func (m textModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyEnter:
			if m.textInput.Value() == "" {
				m.textInput.SetValue(m.textInput.Placeholder)
				return m, tea.Quit
			}
			return m, tea.Quit
		case tea.KeyCtrlC, tea.KeyEsc:
			return m, tea.Quit
		}

	// We handle errors just like any other message
	case errMsg:
		m.err = msg
		return m, nil
	}

	m.textInput, cmd = m.textInput.Update(msg)
	return m, cmd
}

func (m textModel) View() string {
	return fmt.Sprintf(
		"%s\n%s\n",
		m.question,
		m.textInput.View(),
	) + "\n"
}

// text area model section

func askMultiQuestionAndGetAnswer(question, placeholder string) []string {
	initInfo := createTextAreaModel(question, placeholder)
	m, err := tea.NewProgram(initInfo).Run()
	if err != nil {
		log.Fatal(err)
	}
	return strings.Split(m.(textAreaModel).textArea.Value(), "\n")
}

func createTextAreaModel(question, placeholder string) textAreaModel {
	ti := textarea.New()
	ti.Placeholder = placeholder
	ti.Focus()

	return textAreaModel{
		question: question,
		textArea: ti,
		err:      nil,
	}
}

func (m textAreaModel) Init() tea.Cmd {
	return textarea.Blink
}

func (m textAreaModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyEsc:
			if m.textArea.Focused() {
				m.textArea.Blur()
			}
		case tea.KeyCtrlC:
			return m, tea.Quit
		default:
			if !m.textArea.Focused() {
				cmd = m.textArea.Focus()
				cmds = append(cmds, cmd)
			}
		}

	// We handle errors just like any other message
	case errMsg:
		m.err = msg
		return m, nil
	}

	m.textArea, cmd = m.textArea.Update(msg)
	cmds = append(cmds, cmd)
	return m, tea.Batch(cmds...)
}

func (m textAreaModel) View() string {
	return fmt.Sprintf(
		"%s\n\n%s\n\n%s",
		m.question,
		m.textArea.View(),
		"(ctrl+c to quit)",
	) + "\n\n"
}
