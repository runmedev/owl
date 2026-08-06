package cmd

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
	"text/tabwriter"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

type StoreClient interface {
	Snapshot(context.Context, SnapshotRequest) (*SnapshotResult, error)
	Source(context.Context, SourceRequest) (*SourceResult, error)
	Check(context.Context, CheckRequest) (*CheckResult, error)
	Type(context.Context, TypeRequest) (*TypeResult, error)
	Resolve(context.Context, ResolveRequest) (*ResolveResult, error)
	ApplyPromptAnswers(context.Context, []PromptAnswer) (*ResolveResult, error)
	ProjectSpec(context.Context, ProjectSpecRequest) (*ProjectSpecResult, error)
}

type StoreCommandOptions struct {
	ClientFactory              func(*cobra.Command) (StoreClient, error)
	ConfigureSnapshotCommand   func(*cobra.Command)
	ConfigureSourceCommand     func(*cobra.Command)
	ConfigureCheckCommand      func(*cobra.Command)
	ConfigureTypeCommand       func(*cobra.Command)
	ConfigureResolveCommand    func(*cobra.Command)
	ConfigureProjectCommand    func(*cobra.Command)
	PromptInput                PromptInputRunner
	Hidden                     bool
	InsecureModeEnabled        func() bool
	DefineSnapshotInsecureFlag bool
}

type PromptInputRunner func(io.Reader, io.Writer, PromptAction) (string, error)

type SnapshotRequest struct {
	Limit       int
	Reveal      bool
	All         bool
	Insecure    bool
	Interactive bool
}

type SnapshotResult struct {
	Envs []SnapshotEnv
}

type SnapshotEnv struct {
	Name        string
	Value       string
	Description string
	Type        string
	Field       string
	Source      string
	Explicit    bool
	Visibility  string
	Diagnostics []string
	Invalid     bool
}

type SourceRequest struct {
	Export      bool
	Insecure    bool
	Interactive bool
}

type SourceResult struct {
	Envs []string
}

type CheckRequest struct {
	Details     bool
	Interactive bool
}

type CheckResult struct {
	OK          bool
	Diagnostics []string
	Checked     int
}

type TypeRequest struct {
	SpecPath string
	Format   string
	Output   string
	Fix      bool
	All      bool
}

type TypeResult struct {
	Proposals []TypeProposal
	Rendered  string
}

type ResolveRequest struct {
	Interactive bool
}

type ResolveResult struct {
	Attempts []ResolverAttempt
	Actions  []ResolverAction
}

type ResolverAttempt struct {
	ResolverID    string
	ProjectionKey string
	Outcome       string
	Message       string
}

type ResolverAction struct {
	Type   string
	Prompt *PromptAction
}

type PromptAction struct {
	NeedID        string
	ProjectionKey string
	Label         string
	Description   string
	Sensitive     bool
	Required      bool
	AllowEmpty    bool
}

type PromptAnswer struct {
	NeedID string
	Value  string
}

type ProjectSpecRequest struct {
	ConfigPath string
	Output     string
	Write      bool
}

type ProjectSpecResult struct {
	Rendered string
}

type TypeProposal struct {
	Key           string
	CurrentType   string
	SuggestedType string
	Confidence    string
	Reason        string
	Description   string
	Required      bool
}

func NewStoreCommand(opts StoreCommandOptions) *cobra.Command {
	if opts.InsecureModeEnabled == nil {
		opts.InsecureModeEnabled = func() bool { return false }
	}

	cmd := cobra.Command{
		Hidden: opts.Hidden,
		Use:    "store",
		Short:  "Owl store",
		Long:   "Owl Store",
	}

	cmd.AddCommand(NewStoreCommands(opts)...)

	return &cmd
}

func NewStoreCommands(opts StoreCommandOptions) []*cobra.Command {
	if opts.InsecureModeEnabled == nil {
		opts.InsecureModeEnabled = func() bool { return false }
	}

	return []*cobra.Command{
		newSnapshotCommand(opts),
		newSourceCommand(opts),
		newCheckCommand(opts),
		newTypeCommand(opts),
		newResolveCommand(opts),
	}
}

func newSnapshotCommand(opts StoreCommandOptions) *cobra.Command {
	var req SnapshotRequest

	cmd := cobra.Command{
		Hidden: opts.Hidden,
		Use:    "snapshot",
		Short:  "Takes a snapshot of the smart env store",
		Long:   "Inspects environment variables and returns a snapshot of the smart env store.",
		RunE: func(cmd *cobra.Command, args []string) error {
			if !opts.DefineSnapshotInsecureFlag && opts.InsecureModeEnabled() {
				req.Insecure = true
			}
			if req.Reveal && (!req.Insecure || !opts.InsecureModeEnabled()) {
				return errors.New("must be run in insecure mode to prevent misuse; enable by adding --insecure flag")
			}

			client, err := opts.client(cmd)
			if err != nil {
				return err
			}
			if err := resolveInteractively(cmd, opts, client, req.Interactive); err != nil {
				return err
			}

			result, err := client.Snapshot(cmd.Context(), req)
			if err != nil {
				return err
			}

			if err := renderSnapshot(cmd.OutOrStdout(), result, req); err != nil {
				return errors.Wrap(err, "failed to render")
			}
			if snapshotHasInvalidRows(result.Envs, req) {
				_, _ = fmt.Fprintln(cmd.ErrOrStderr(), "warning: snapshot contains invalid state; run `owl check` for details")
			}
			return nil
		},
	}

	cmd.Flags().IntVar(&req.Limit, "limit", 50, "Limit the number of lines")
	cmd.Flags().BoolVarP(&req.All, "all", "A", false, "Show all lines")
	cmd.Flags().BoolVarP(&req.Reveal, "reveal", "r", false, "Reveal hidden values")
	cmd.Flags().BoolVar(&req.Interactive, "interactive", false, "Interactively resolve unresolved values")
	if opts.DefineSnapshotInsecureFlag {
		cmd.Flags().BoolVar(&req.Insecure, "insecure", false, "Explicitly allow revealing hidden values")
	}
	if opts.ConfigureSnapshotCommand != nil {
		opts.ConfigureSnapshotCommand(&cmd)
	}

	return &cmd
}

func newSourceCommand(opts StoreCommandOptions) *cobra.Command {
	var req SourceRequest

	cmd := cobra.Command{
		Use:   "source",
		Short: "Source environment variables",
		Long:  "Source environment variables",
		RunE: func(cmd *cobra.Command, args []string) error {
			if !req.Insecure {
				return errors.New("must be run in insecure mode to prevent misuse; enable by adding --insecure flag")
			}

			client, err := opts.client(cmd)
			if err != nil {
				return err
			}
			if err := resolveInteractively(cmd, opts, client, req.Interactive); err != nil {
				return err
			}

			result, err := client.Source(cmd.Context(), req)
			if err != nil {
				return err
			}

			return renderSource(cmd.OutOrStdout(), result, req)
		},
	}

	cmd.Flags().BoolVarP(&req.Export, "export", "", false, "export variables")
	cmd.Flags().BoolVar(&req.Insecure, "insecure", false, "Explicitly allow delicate operations to prevent misuse")
	cmd.Flags().BoolVar(&req.Interactive, "interactive", false, "Interactively resolve unresolved values")
	if opts.ConfigureSourceCommand != nil {
		opts.ConfigureSourceCommand(&cmd)
	}

	return &cmd
}

func newCheckCommand(opts StoreCommandOptions) *cobra.Command {
	var req CheckRequest

	cmd := cobra.Command{
		Hidden: opts.Hidden,
		Use:    "check",
		Short:  "Validates smart store",
		Long:   "Validates smart store, exiting with success or displaying errors.",
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := opts.client(cmd)
			if err != nil {
				return err
			}
			if err := resolveInteractively(cmd, opts, client, req.Interactive); err != nil {
				return err
			}

			result, err := client.Check(cmd.Context(), req)
			if err != nil {
				return err
			}

			if len(result.Diagnostics) == 0 {
				_, err = fmt.Fprintf(cmd.OutOrStdout(), "ok: %d variables checked, 0 errors, 0 warnings\n", result.Checked)
				return err
			}
			for _, diagnostic := range result.Diagnostics {
				if _, err = fmt.Fprintln(cmd.OutOrStdout(), diagnostic); err != nil {
					return err
				}
			}
			if !result.OK {
				return errSilentExit
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&req.Details, "details", false, "Show detailed validation errors")
	cmd.Flags().BoolVar(&req.Interactive, "interactive", false, "Interactively resolve unresolved values")
	if opts.ConfigureCheckCommand != nil {
		opts.ConfigureCheckCommand(&cmd)
	}

	return &cmd
}

func newTypeCommand(opts StoreCommandOptions) *cobra.Command {
	var req TypeRequest

	cmd := cobra.Command{
		Hidden: opts.Hidden,
		Use:    "type",
		Short:  "Proposes missing env types",
		Long:   "Proposes missing primitive env types for a dotenv spec.",
		RunE: func(cmd *cobra.Command, args []string) error {
			if req.Format == "" {
				req.Format = "table"
			}
			if req.Fix && req.Output != "" {
				return errors.New("use either --fix or --output, not both")
			}

			client, err := opts.client(cmd)
			if err != nil {
				return err
			}

			result, err := client.Type(cmd.Context(), req)
			if err != nil {
				return err
			}

			return errors.Wrap(renderType(cmd.OutOrStdout(), result, req), "failed to render")
		},
	}

	cmd.Flags().StringVar(&req.SpecPath, "spec", ".env.spec", "Env spec file to type")
	cmd.Flags().StringVar(&req.Format, "format", "table", "Output format: table or dotenv-spec")
	cmd.Flags().StringVar(&req.Output, "output", "", "Write the changed env spec to a file, or '-' for stdout")
	cmd.Flags().BoolVar(&req.Fix, "fix", false, "Append proposed types to the spec file")
	cmd.Flags().BoolVar(&req.All, "all", false, "Show envs without type suggestions")
	if opts.ConfigureTypeCommand != nil {
		opts.ConfigureTypeCommand(&cmd)
	}

	return &cmd
}

func newResolveCommand(opts StoreCommandOptions) *cobra.Command {
	var req ResolveRequest

	cmd := cobra.Command{
		Hidden: opts.Hidden,
		Use:    "resolve",
		Short:  "Resolve missing environment values",
		Long:   "Resolve missing environment values.",
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := opts.client(cmd)
			if err != nil {
				return err
			}
			result, err := client.Resolve(cmd.Context(), req)
			if err != nil {
				return err
			}
			if !req.Interactive {
				return renderResolve(cmd.OutOrStdout(), result)
			}
			promptInput := opts.PromptInput
			if promptInput == nil {
				promptInput = runCharmPromptInput
			}
			answers, err := readPromptAnswers(cmd.InOrStdin(), cmd.ErrOrStderr(), result.Actions, promptInput)
			if err != nil {
				return err
			}
			if len(answers) == 0 {
				return renderResolve(cmd.OutOrStdout(), result)
			}
			applied, err := client.ApplyPromptAnswers(cmd.Context(), answers)
			if err != nil {
				return err
			}
			if _, err := fmt.Fprintf(cmd.OutOrStdout(), "resolved %d interactive values\n", len(applied.Attempts)); err != nil {
				return err
			}
			return nil
		},
	}

	cmd.Flags().BoolVar(&req.Interactive, "interactive", false, "Interactively resolve unresolved values")
	if opts.ConfigureResolveCommand != nil {
		opts.ConfigureResolveCommand(&cmd)
	}

	return &cmd
}

func (opts StoreCommandOptions) client(cmd *cobra.Command) (StoreClient, error) {
	if opts.ClientFactory == nil {
		return nil, errors.New("store client factory is required")
	}
	return opts.ClientFactory(cmd)
}

func resolveInteractively(cmd *cobra.Command, opts StoreCommandOptions, client StoreClient, interactive bool) error {
	if !interactive {
		return nil
	}
	result, err := client.Resolve(cmd.Context(), ResolveRequest{Interactive: true})
	if err != nil {
		return err
	}
	promptInput := opts.PromptInput
	if promptInput == nil {
		promptInput = runCharmPromptInput
	}
	answers, err := readPromptAnswers(cmd.InOrStdin(), cmd.ErrOrStderr(), result.Actions, promptInput)
	if err != nil {
		return err
	}
	if len(answers) == 0 {
		return nil
	}
	_, err = client.ApplyPromptAnswers(cmd.Context(), answers)
	return err
}

func renderSnapshot(w io.Writer, result *SnapshotResult, req SnapshotRequest) error {
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	if _, err := fmt.Fprintln(tw, "NAME\tVALUE\tTYPE\tSOURCE\tSTATUS\tDESCRIPTION"); err != nil {
		return err
	}

	envs := snapshotEnvsForRender(result.Envs, req)
	for i, env := range envs {
		if i >= req.Limit && !req.All {
			break
		}

		strippedVal := strings.ReplaceAll(strings.ReplaceAll(env.Value, "\n", " "), "\r", "")
		if _, err := fmt.Fprintf(
			tw,
			"%s\t%s\t%s\t%s\t%s\t%s\n",
			env.Name,
			strippedVal,
			env.Type,
			env.Source,
			env.Visibility,
			env.Description,
		); err != nil {
			return err
		}
	}

	return tw.Flush()
}

func renderResolve(w io.Writer, result *ResolveResult) error {
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	if _, err := fmt.Fprintln(tw, "KEY\tRESOLVER\tOUTCOME\tMESSAGE"); err != nil {
		return err
	}
	for _, attempt := range result.Attempts {
		if _, err := fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n", attempt.ProjectionKey, attempt.ResolverID, attempt.Outcome, attempt.Message); err != nil {
			return err
		}
	}
	for _, action := range result.Actions {
		if action.Prompt == nil {
			continue
		}
		if _, err := fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n", action.Prompt.ProjectionKey, "core/interactive", action.Type, action.Prompt.Label); err != nil {
			return err
		}
	}
	return tw.Flush()
}

func readPromptAnswers(r io.Reader, prompt io.Writer, actions []ResolverAction, promptInput PromptInputRunner) ([]PromptAnswer, error) {
	var answers []PromptAnswer
	for _, action := range actions {
		if action.Prompt == nil || action.Type != "interactive" {
			continue
		}
		value, err := promptInput(r, prompt, *action.Prompt)
		if err != nil {
			return nil, err
		}
		if value == "" && !action.Prompt.AllowEmpty {
			return nil, errors.Errorf("%s cannot be empty", action.Prompt.ProjectionKey)
		}
		answers = append(answers, PromptAnswer{NeedID: action.Prompt.NeedID, Value: value})
	}
	return answers, nil
}

func runCharmPromptInput(r io.Reader, w io.Writer, prompt PromptAction) (string, error) {
	if !isTerminalReader(r) {
		return readLinePromptInput(r, w, prompt)
	}
	model := newPromptInputModel(prompt)
	program := tea.NewProgram(model, tea.WithInput(r), tea.WithOutput(w))
	finished, err := program.Run()
	if err != nil {
		return "", err
	}
	next, ok := finished.(promptInputModel)
	if !ok {
		return "", errors.New("prompt returned unexpected model")
	}
	return next.value, nil
}

func readLinePromptInput(r io.Reader, w io.Writer, prompt PromptAction) (string, error) {
	label := prompt.Label
	if label == "" {
		label = prompt.ProjectionKey
	}
	if _, err := fmt.Fprintf(w, "%s: ", label); err != nil {
		return "", err
	}
	value, err := bufio.NewReader(r).ReadString('\n')
	if err != nil && err != io.EOF {
		return "", err
	}
	return strings.TrimRight(value, "\r\n"), nil
}

func isTerminalReader(r io.Reader) bool {
	file, ok := r.(*os.File)
	if !ok {
		return false
	}
	info, err := file.Stat()
	if err != nil {
		return false
	}
	return info.Mode()&os.ModeCharDevice != 0
}

type promptInputModel struct {
	label string
	input textinput.Model
	value string
	done  bool
}

func newPromptInputModel(prompt PromptAction) promptInputModel {
	label := prompt.Label
	if label == "" {
		label = prompt.ProjectionKey
	}
	input := textinput.New()
	input.Prompt = ""
	input.Placeholder = prompt.Description
	if prompt.Sensitive {
		input.EchoMode = textinput.EchoPassword
		input.EchoCharacter = '*'
	}
	input.Focus()
	return promptInputModel{label: label, input: input}
}

func (m promptInputModel) Init() tea.Cmd {
	return textinput.Blink
}

func (m promptInputModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	if key, ok := msg.(tea.KeyMsg); ok && key.Type == tea.KeyEnter {
		m.value = m.input.Value()
		m.done = true
		return m, tea.Quit
	}
	return m, cmd
}

func (m promptInputModel) View() string {
	if m.done {
		return ""
	}
	return fmt.Sprintf("%s: %s\n", m.label, m.input.View())
}

func snapshotHasInvalidRows(envs []SnapshotEnv, req SnapshotRequest) bool {
	for i, env := range snapshotEnvsForRender(envs, req) {
		if i >= req.Limit && !req.All {
			break
		}
		if env.Invalid {
			return true
		}
	}
	return false
}

func snapshotEnvsForRender(envs []SnapshotEnv, req SnapshotRequest) []SnapshotEnv {
	explicit := make([]SnapshotEnv, 0, len(envs))
	inherited := make([]SnapshotEnv, 0, len(envs))
	for _, env := range envs {
		if env.Explicit {
			explicit = append(explicit, env)
			continue
		}
		inherited = append(inherited, env)
	}
	sortSnapshotEnvs(explicit)
	sortSnapshotEnvs(inherited)
	if !req.All {
		return explicit
	}
	return append(explicit, inherited...)
}

func sortSnapshotEnvs(envs []SnapshotEnv) {
	sort.SliceStable(envs, func(i, j int) bool {
		return envs[i].Name < envs[j].Name
	})
}

func renderSource(w io.Writer, result *SourceResult, req SourceRequest) error {
	for _, kv := range result.Envs {
		parts := strings.Split(kv, "=")
		if len(parts) < 2 {
			return errors.Errorf("invalid key-value pair: %s", kv)
		}

		envVar := fmt.Sprintf("%s=%q", parts[0], strings.Join(parts[1:], "="))
		if req.Export {
			envVar = fmt.Sprintf("export %s", envVar)
		}

		if _, err := fmt.Fprintf(w, "%s\n", envVar); err != nil {
			return err
		}
	}

	return nil
}

func renderType(w io.Writer, result *TypeResult, req TypeRequest) error {
	if req.Output == "-" {
		if _, err := fmt.Fprint(w, result.Rendered); err != nil {
			return err
		}
		return nil
	}
	if req.Format == "dotenv-spec" {
		if _, err := fmt.Fprint(w, result.Rendered); err != nil {
			return err
		}
		return nil
	}
	if req.Format != "table" {
		return errors.Errorf("unsupported type output format: %s", req.Format)
	}

	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	if _, err := fmt.Fprintln(tw, "KEY\tCURRENT\tSUGGESTED\tCONFIDENCE\tREASON"); err != nil {
		return err
	}
	for _, proposal := range result.Proposals {
		suggestedType := proposal.SuggestedType
		if suggestedType == "" {
			suggestedType = "-"
		}
		if _, err := fmt.Fprintf(
			tw,
			"%s\t%s\t%s\t%s\t%s\n",
			proposal.Key,
			proposal.CurrentType,
			suggestedType,
			proposal.Confidence,
			proposal.Reason,
		); err != nil {
			return err
		}
	}
	return tw.Flush()
}
