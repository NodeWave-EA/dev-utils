// DESC: Simplify and manage Docker and Docker Compose workflows with interactive helpers
package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"syscall"
)

type Style struct {
	Bold   string
	Dim    string
	Cyan   string
	Green  string
	Yellow string
	Red    string
	Reset  string
}

func newStyle() Style {
	if os.Getenv("NO_COLOR") != "" || !isStdoutTTY() {
		return Style{}
	}
	return Style{
		Bold:   "\033[1m",
		Dim:    "\033[2m",
		Cyan:   "\033[36m",
		Green:  "\033[32m",
		Yellow: "\033[33m",
		Red:    "\033[31m",
		Reset:  "\033[0m",
	}
}

func isStdoutTTY() bool {
	info, err := os.Stdout.Stat()
	return err == nil && (info.Mode()&os.ModeCharDevice) != 0
}

func isStdinTTY() bool {
	info, err := os.Stdin.Stat()
	return err == nil && (info.Mode()&os.ModeCharDevice) != 0
}

func isInteractive() bool {
	return isStdoutTTY() && isStdinTTY()
}

type Container struct {
	ID      string
	Name    string
	Image   string
	State   string
	Status  string
	Ports   string
	Project string
	Service string
}

type ComposeRuntime struct {
	Bin         string
	PrefixArgs  []string
	ComposeFile string
	ProjectDir  string
}

var (
	st     Style
	docker string
)

func main() {
	st = newStyle()

	var err error
	docker, err = exec.LookPath("docker")
	if err != nil {
		fatal("docker binary not found in $PATH")
	}

	args := os.Args[1:]
	if len(args) == 0 {
		if isInteractive() {
			runInteractiveHome()
			return
		}
		printHelp()
		return
	}

	sub := args[0]
	rest := args[1:]

	switch sub {
	case "-h", "--help", "help":
		printHelp()
	case "ps", "ls", "status":
		cmdPS(rest)
	case "up":
		cmdUp(rest)
	case "down":
		cmdDown(rest)
	case "rebuild":
		cmdRebuild(rest)
	case "build":
		cmdBuild(rest)
	case "sh", "shell", "bash", "enter":
		cmdShell(rest)
	case "exec":
		cmdExec(rest)
	case "logs", "log":
		cmdLogs(rest)
	case "restart", "stop", "start":
		cmdLifecycle(sub, rest)
	case "stats":
		cmdStats(rest)
	case "env":
		cmdEnv(rest)
	case "ip", "net":
		cmdIP(rest)
	case "clean", "prune":
		cmdClean(rest)
	case "ctx", "context":
		cmdContext(rest)
	case "compose", "dc":
		cmdComposePassthrough(rest)
	default:
		// Transparent pass-through for every native `docker` command/flag
		execReplace(docker, append([]string{"docker"}, args...))
	}
}

func fatal(msg string) {
	fmt.Fprintf(os.Stderr, "%sError:%s %s\n", st.Red, st.Reset, msg)
	os.Exit(1)
}

func exitWithErr(err error) {
	if err == nil {
		return
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		os.Exit(exitErr.ExitCode())
	}
	fatal(err.Error())
}

func printHelp() {
	fmt.Printf("%sdev docker%s — Simplified Docker & Docker Compose manager\n\n", st.Bold, st.Reset)
	fmt.Printf("%sUsage:%s\n", st.Bold, st.Reset)
	fmt.Println("  dev docker                       Interactive dashboard & quick launcher")
	fmt.Println("  dev docker <shortcut> [args]     Simplified container & Compose workflows")
	fmt.Println("  dev docker dc <args...>          Direct pass-through to 'docker compose'")
	fmt.Println("  dev docker <any-docker-cmd>      Full pass-through to 'docker'")
	fmt.Println()

	fmt.Printf("%sCompose Shortcuts:%s\n", st.Bold, st.Reset)
	fmt.Printf("  %s%-16s%s Start Compose stack in detached mode (-d by default; use --fg to attach)\n", st.Cyan, "up [services]", st.Reset)
	fmt.Printf("  %s%-16s%s Stop & remove Compose stack (-v to remove volumes with confirmation)\n", st.Cyan, "down [-v]", st.Reset)
	fmt.Printf("  %s%-16s%s Rebuild images & force-recreate Compose services (up -d --build --force-recreate)\n", st.Cyan, "rebuild [svc]", st.Reset)
	fmt.Printf("  %s%-16s%s Build Compose services (or current Dockerfile if outside a Compose project)\n", st.Cyan, "build [svc]", st.Reset)
	fmt.Printf("  %s%-16s%s Pass any command directly to docker compose (alias: compose)\n", st.Cyan, "dc <args...>", st.Reset)
	fmt.Println()

	fmt.Printf("%sContainer & Debugging Shortcuts:%s\n", st.Bold, st.Reset)
	fmt.Printf("  %s%-16s%s Clean status table of containers, ports & Compose services (-a for stopped)\n", st.Cyan, "ps [-a]", st.Reset)
	fmt.Printf("  %s%-16s%s Open interactive shell (auto-detects bash/zsh/ash/sh) or run a command\n", st.Cyan, "sh [target] [cmd]", st.Reset)
	fmt.Printf("  %s%-16s%s Tail & follow logs (defaults to -f --tail 100 in TTY; interactive picker)\n", st.Cyan, "logs [target]", st.Reset)
	fmt.Printf("  %s%-16s%s Restart, stop, or start a Compose service or container (supports fuzzy names)\n", st.Cyan, "restart|stop|start", st.Reset)
	fmt.Printf("  %s%-16s%s Show container/service environment variables (masks secrets unless --reveal)\n", st.Cyan, "env [target]", st.Reset)
	fmt.Printf("  %s%-16s%s Show container IP addresses, networks & port mappings\n", st.Cyan, "ip [target]", st.Reset)
	fmt.Printf("  %s%-16s%s Live or one-shot (--no-stream) CPU & memory stats\n", st.Cyan, "stats [target]", st.Reset)
	fmt.Printf("  %s%-16s%s Interactive disk usage check & safe/deep prune cleanup\n", st.Cyan, "clean [--safe|--deep]", st.Reset)
	fmt.Printf("  %s%-16s%s Inspect or interactively switch Docker contexts\n", st.Cyan, "ctx [name]", st.Reset)
}

// ============================================================================
// Runtime & Project Discovery
// ============================================================================

func detectCompose() ComposeRuntime {
	rt := ComposeRuntime{}

	// Prefer Docker Compose v2 plugin (`docker compose`), fallback to `docker-compose`
	if err := exec.Command(docker, "compose", "version").Run(); err == nil {
		rt.Bin = docker
		rt.PrefixArgs = []string{"compose"}
	} else if legacy, err := exec.LookPath("docker-compose"); err == nil {
		rt.Bin = legacy
	}

	// Search current directory and parent directories for a Compose file
	candidates := []string{
		"compose.yaml",
		"compose.yml",
		"docker-compose.yaml",
		"docker-compose.yml",
	}

	dir, err := os.Getwd()
	if err != nil {
		return rt
	}

	for {
		for _, name := range candidates {
			full := filepath.Join(dir, name)
			if info, statErr := os.Stat(full); statErr == nil && !info.IsDir() {
				rt.ComposeFile = full
				rt.ProjectDir = dir
				return rt
			}
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}

	return rt
}

func requireCompose() ComposeRuntime {
	rt := detectCompose()
	if rt.Bin == "" {
		fatal("neither 'docker compose' plugin nor 'docker-compose' binary was found")
	}
	if rt.ComposeFile == "" {
		fatal("no compose.yaml or docker-compose.yml found in current or parent directories")
	}
	return rt
}

func composeServices(rt ComposeRuntime) []string {
	if rt.Bin == "" || rt.ComposeFile == "" {
		return nil
	}
	args := append([]string{}, rt.PrefixArgs...)
	args = append(args, "-f", rt.ComposeFile, "config", "--services")
	out, err := exec.Command(rt.Bin, args...).Output()
	if err != nil {
		return nil
	}
	var services []string
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			services = append(services, line)
		}
	}
	return services
}

func listContainers(all bool) ([]Container, error) {
	args := []string{
		"ps",
		"--no-trunc",
		"--format",
		`{{.ID}}\t{{.Names}}\t{{.Image}}\t{{.State}}\t{{.Status}}\t{{.Ports}}\t{{.Label "com.docker.compose.project"}}\t{{.Label "com.docker.compose.service"}}`,
	}
	if all {
		args = append(args, "-a")
	}

	out, err := exec.Command(docker, args...).Output()
	if err != nil {
		return nil, err
	}

	raw := strings.TrimSpace(string(out))
	if raw == "" {
		return nil, nil
	}

	var list []Container
	for _, line := range strings.Split(raw, "\n") {
		cols := strings.Split(line, "\t")
		if len(cols) < 8 {
			continue
		}
		shortID := cols[0]
		if len(shortID) > 12 {
			shortID = shortID[:12]
		}
		list = append(list, Container{
			ID:      shortID,
			Name:    cols[1],
			Image:   cols[2],
			State:   cols[3],
			Status:  cols[4],
			Ports:   cols[5],
			Project: cols[6],
			Service: cols[7],
		})
	}
	return list, nil
}

// ============================================================================
// Interactive Picker & Fuzzy Target Resolution
// ============================================================================

type PickOption struct {
	Value string
	Label string
}

func promptConfirm(question string, defaultYes bool) bool {
	if !isInteractive() {
		return defaultYes
	}
	hint := "[y/N]"
	if defaultYes {
		hint = "[Y/n]"
	}
	fmt.Printf("%s%s %s:%s ", st.Yellow, question, hint, st.Reset)
	reader := bufio.NewReader(os.Stdin)
	line, err := reader.ReadString('\n')
	if err != nil {
		return false
	}
	ans := strings.ToLower(strings.TrimSpace(line))
	if ans == "" {
		return defaultYes
	}
	return ans == "y" || ans == "yes"
}

func interactivePick(prompt string, options []PickOption) (string, bool) {
	if len(options) == 0 {
		return "", false
	}
	if !isInteractive() {
		return "", false
	}

	// Use fzf if available for fast fuzzy selection
	if fzfPath, err := exec.LookPath("fzf"); err == nil {
		var input bytes.Buffer
		for _, opt := range options {
			fmt.Fprintf(&input, "%s\t%s\n", opt.Value, opt.Label)
		}
		cmd := exec.Command(fzfPath,
			"--height=45%",
			"--reverse",
			"--delimiter=\t",
			"--with-nth=2",
			"--prompt="+prompt+" > ",
		)
		cmd.Stdin = &input
		cmd.Stderr = os.Stderr
		out, runErr := cmd.Output()
		if runErr != nil {
			return "", false
		}
		chosen := strings.TrimSpace(string(out))
		if chosen == "" {
			return "", false
		}
		parts := strings.SplitN(chosen, "\t", 2)
		return parts[0], true
	}

	// Built-in numbered fallback menu
	fmt.Printf("%s%s%s\n", st.Bold, prompt, st.Reset)
	for i, opt := range options {
		fmt.Printf("  %s%2d)%s %s\n", st.Cyan, i+1, st.Reset, opt.Label)
	}
	fmt.Printf("\n%sSelect number (or Enter to cancel):%s ", st.Bold, st.Reset)

	reader := bufio.NewReader(os.Stdin)
	line, err := reader.ReadString('\n')
	if err != nil {
		return "", false
	}
	line = strings.TrimSpace(line)
	if line == "" {
		return "", false
	}
	idx, err := strconv.Atoi(line)
	if err != nil || idx < 1 || idx > len(options) {
		return "", false
	}
	return options[idx-1].Value, true
}

func resolveContainer(query string, includeStopped bool, allowAllOption bool) (Container, bool) {
	containers, err := listContainers(includeStopped)
	if err != nil {
		exitWithErr(err)
	}
	if len(containers) == 0 {
		if includeStopped {
			fatal("no Docker containers found")
		}
		fatal("no running Docker containers found (use -a to include stopped containers)")
	}

	if query == "" {
		var opts []PickOption
		if allowAllOption {
			opts = append(opts, PickOption{
				Value: "__ALL__",
				Label: fmt.Sprintf("%s[ALL]%s  All containers / Compose services", st.Bold, st.Reset),
			})
		}
		for _, c := range containers {
			svcTag := ""
			if c.Service != "" {
				svcTag = fmt.Sprintf(" [%s/%s]", c.Project, c.Service)
			}
			opts = append(opts, PickOption{
				Value: c.Name,
				Label: fmt.Sprintf("%-26s %-22s %s%s", c.Name, c.Status, c.Image, svcTag),
			})
		}
		picked, ok := interactivePick("Select container", opts)
		if !ok {
			os.Exit(0)
		}
		if picked == "__ALL__" {
			return Container{Name: "__ALL__"}, true
		}
		query = picked
	}

	q := strings.ToLower(strings.TrimSpace(query))

	// 1. Exact match on container name, ID, or Compose service name
	for _, c := range containers {
		if strings.ToLower(c.Name) == q || strings.ToLower(c.ID) == q || strings.ToLower(c.Service) == q {
			return c, true
		}
	}

	// 2. Prefix / substring match on name, service, image, or ID
	var matches []Container
	for _, c := range containers {
		if strings.Contains(strings.ToLower(c.Name), q) ||
			strings.Contains(strings.ToLower(c.Service), q) ||
			strings.HasPrefix(strings.ToLower(c.ID), q) ||
			strings.Contains(strings.ToLower(c.Image), q) {
			matches = append(matches, c)
		}
	}

	if len(matches) == 1 {
		return matches[0], true
	}
	if len(matches) > 1 {
		var opts []PickOption
		for _, c := range matches {
			opts = append(opts, PickOption{
				Value: c.Name,
				Label: fmt.Sprintf("%-26s %-22s %s", c.Name, c.Status, c.Image),
			})
		}
		picked, ok := interactivePick(fmt.Sprintf("Multiple matches for '%s'", query), opts)
		if !ok {
			os.Exit(0)
		}
		for _, c := range matches {
			if c.Name == picked {
				return c, true
			}
		}
	}

	fatal(fmt.Sprintf("no container or service matching '%s'", query))
	return Container{}, false
}

// ============================================================================
// Subcommands
// ============================================================================

func runInteractiveHome() {
	rt := detectCompose()
	containers, _ := listContainers(false)

	fmt.Printf("%sdev docker%s — Interactive Control Center\n", st.Bold, st.Reset)
	if rt.ComposeFile != "" {
		fmt.Printf("  Compose file : %s%s%s\n", st.Cyan, rt.ComposeFile, st.Reset)
	}
	fmt.Printf("  Running      : %s%d container(s)%s\n\n", st.Green, len(containers), st.Reset)

	var actions []PickOption
	actions = append(actions,
		PickOption{Value: "ps", Label: "ps       — List running & stopped containers"},
		PickOption{Value: "logs", Label: "logs     — Tail & follow container or service logs"},
		PickOption{Value: "sh", Label: "sh       — Open an interactive shell inside a container"},
	)
	if rt.ComposeFile != "" {
		actions = append(actions,
			PickOption{Value: "up", Label: "up       — Start Compose stack in background (up -d)"},
			PickOption{Value: "rebuild", Label: "rebuild  — Rebuild & recreate Compose services"},
			PickOption{Value: "down", Label: "down     — Stop & remove Compose stack"},
		)
	}
	actions = append(actions,
		PickOption{Value: "restart", Label: "restart  — Restart a container or Compose service"},
		PickOption{Value: "stats", Label: "stats    — View live CPU & memory usage"},
		PickOption{Value: "ip", Label: "ip       — Inspect container IP addresses & port mappings"},
		PickOption{Value: "env", Label: "env      — Inspect container environment variables"},
		PickOption{Value: "clean", Label: "clean    — Reclaim disk space (prune containers/images/cache)"},
		PickOption{Value: "ctx", Label: "ctx      — Switch Docker context"},
	)

	choice, ok := interactivePick("Choose action", actions)
	if !ok {
		return
	}

	switch choice {
	case "ps":
		cmdPS([]string{"-a"})
	case "logs":
		cmdLogs(nil)
	case "sh":
		cmdShell(nil)
	case "up":
		cmdUp(nil)
	case "rebuild":
		cmdRebuild(nil)
	case "down":
		cmdDown(nil)
	case "restart":
		cmdLifecycle("restart", nil)
	case "stats":
		cmdStats(nil)
	case "ip":
		cmdIP(nil)
	case "env":
		cmdEnv(nil)
	case "clean":
		cmdClean(nil)
	case "ctx":
		cmdContext(nil)
	}
}

func cmdPS(args []string) {
	showAll := false
	asJSON := false
	for _, arg := range args {
		switch arg {
		case "-a", "--all":
			showAll = true
		case "--json":
			asJSON = true
		default:
			execReplace(docker, append([]string{"docker", "ps"}, args...))
		}
	}

	containers, err := listContainers(showAll)
	if err != nil {
		exitWithErr(err)
	}

	if asJSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		_ = enc.Encode(containers)
		return
	}

	if len(containers) == 0 {
		fmt.Println("No running containers. Use 'dev docker ps -a' to include stopped containers.")
		return
	}

	fmt.Printf("%s%-24s %-16s %-26s %-22s %s%s\n",
		st.Bold, "NAME", "SERVICE", "IMAGE", "STATUS", "PORTS", st.Reset)

	for _, c := range containers {
		svc := "-"
		if c.Service != "" {
			svc = c.Service
		}
		stateColor := st.Green
		if c.State != "running" {
			stateColor = st.Yellow
		}
		ports := formatPorts(c.Ports)
		fmt.Printf("%-24s %s%-16s%s %-26s %s%-22s%s %s\n",
			truncate(c.Name, 23),
			st.Cyan, truncate(svc, 15), st.Reset,
			truncate(c.Image, 25),
			stateColor, truncate(c.Status, 21), st.Reset,
			ports,
		)
	}
}

func formatPorts(raw string) string {
	if raw == "" {
		return "-"
	}
	parts := strings.Split(raw, ", ")
	var clean []string
	seen := make(map[string]bool)
	for _, p := range parts {
		p = strings.TrimPrefix(p, "0.0.0.0:")
		p = strings.TrimPrefix(p, ":::")
		if !seen[p] {
			seen[p] = true
			clean = append(clean, p)
		}
	}
	return strings.Join(clean, ", ")
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	if maxLen <= 3 {
		return s[:maxLen]
	}
	return s[:maxLen-3] + "..."
}

func cmdUp(args []string) {
	rt := requireCompose()
	hasDetachOrFG := false
	var filtered []string

	for _, arg := range args {
		switch arg {
		case "--fg", "--attach":
			hasDetachOrFG = true
		case "-d", "--detach":
			hasDetachOrFG = true
			filtered = append(filtered, arg)
		case "-b":
			filtered = append(filtered, "--build")
		default:
			filtered = append(filtered, arg)
		}
	}

	cmdArgs := append([]string{}, rt.PrefixArgs...)
	cmdArgs = append(cmdArgs, "-f", rt.ComposeFile, "up")
	if !hasDetachOrFG {
		cmdArgs = append(cmdArgs, "-d")
	}
	cmdArgs = append(cmdArgs, filtered...)

	runAttached(rt.Bin, cmdArgs)
}

func cmdDown(args []string) {
	rt := requireCompose()
	hasVolumes := false
	autoYes := false
	var filtered []string

	for _, arg := range args {
		switch arg {
		case "-v", "--volumes":
			hasVolumes = true
			filtered = append(filtered, arg)
		case "-y", "--yes":
			autoYes = true
		default:
			filtered = append(filtered, arg)
		}
	}

	if hasVolumes && !autoYes && isInteractive() {
		if !promptConfirm("This will permanently delete named Compose volumes. Continue?", false) {
			fmt.Println("Aborted.")
			return
		}
	}

	cmdArgs := append([]string{}, rt.PrefixArgs...)
	cmdArgs = append(cmdArgs, "-f", rt.ComposeFile, "down")
	cmdArgs = append(cmdArgs, filtered...)

	runAttached(rt.Bin, cmdArgs)
}

func cmdRebuild(args []string) {
	rt := requireCompose()
	cmdArgs := append([]string{}, rt.PrefixArgs...)
	cmdArgs = append(cmdArgs, "-f", rt.ComposeFile, "up", "-d", "--build", "--force-recreate")
	cmdArgs = append(cmdArgs, args...)
	runAttached(rt.Bin, cmdArgs)
}

func cmdBuild(args []string) {
	rt := detectCompose()
	if rt.ComposeFile != "" && rt.Bin != "" {
		cmdArgs := append([]string{}, rt.PrefixArgs...)
		cmdArgs = append(cmdArgs, "-f", rt.ComposeFile, "build")
		cmdArgs = append(cmdArgs, args...)
		runAttached(rt.Bin, cmdArgs)
		return
	}

	if len(args) == 0 {
		cwd, _ := os.Getwd()
		tag := strings.ToLower(filepath.Base(cwd)) + ":latest"
		fmt.Printf("%sBuilding image '%s' from current directory...%s\n", st.Cyan, tag, st.Reset)
		execReplace(docker, []string{"docker", "build", "-t", tag, "."})
	}
	execReplace(docker, append([]string{"docker", "build"}, args...))
}

func cmdShell(args []string) {
	targetQuery := ""
	var customCmd []string
	if len(args) > 0 {
		targetQuery = args[0]
		customCmd = args[1:]
	}

	c, _ := resolveContainer(targetQuery, false, false)

	if len(customCmd) > 0 {
		execArgs := []string{"docker", "exec", "-it", c.Name}
		if !isInteractive() {
			execArgs = []string{"docker", "exec", "-i", c.Name}
		}
		execArgs = append(execArgs, customCmd...)
		execReplace(docker, execArgs)
	}

	// Probe common shells inside the container so Alpine/slim images work seamlessly
	shells := []string{"/bin/bash", "/bin/zsh", "/bin/ash", "/bin/sh"}
	chosenShell := "/bin/sh"
	for _, sh := range shells {
		if err := exec.Command(docker, "exec", c.Name, "test", "-x", sh).Run(); err == nil {
			chosenShell = sh
			break
		}
	}

	fmt.Printf("%sConnecting to %s (%s)...%s\n", st.Dim, c.Name, chosenShell, st.Reset)
	execReplace(docker, []string{"docker", "exec", "-it", c.Name, chosenShell})
}

func cmdExec(args []string) {
	// If user passed explicit flags (like -it, -u, -e), forward verbatim
	if len(args) == 0 || strings.HasPrefix(args[0], "-") {
		execReplace(docker, append([]string{"docker", "exec"}, args...))
	}
	if len(args) == 1 {
		cmdShell(args)
		return
	}
	c, _ := resolveContainer(args[0], false, false)
	execArgs := []string{"docker", "exec"}
	if isInteractive() {
		execArgs = append(execArgs, "-it")
	}
	execArgs = append(execArgs, c.Name)
	execArgs = append(execArgs, args[1:]...)
	execReplace(docker, execArgs)
}

func cmdLogs(args []string) {
	targetQuery := ""
	hasFollowFlag := false
	hasTailFlag := false
	var extraFlags []string

	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "-f" || arg == "--follow" || arg == "--no-follow":
			hasFollowFlag = true
			if arg != "--no-follow" {
				extraFlags = append(extraFlags, arg)
			}
		case arg == "-n" || arg == "--tail":
			hasTailFlag = true
			extraFlags = append(extraFlags, arg)
			if i+1 < len(args) {
				i++
				extraFlags = append(extraFlags, args[i])
			}
		case strings.HasPrefix(arg, "--tail="):
			hasTailFlag = true
			extraFlags = append(extraFlags, arg)
		case strings.HasPrefix(arg, "-"):
			extraFlags = append(extraFlags, arg)
		default:
			if targetQuery == "" {
				targetQuery = arg
			} else {
				extraFlags = append(extraFlags, arg)
			}
		}
	}

	rt := detectCompose()
	c, _ := resolveContainer(targetQuery, true, rt.ComposeFile != "")

	var logFlags []string
	if !hasTailFlag {
		logFlags = append(logFlags, "--tail", "100")
	}
	if !hasFollowFlag && isStdoutTTY() {
		logFlags = append(logFlags, "-f")
	}
	logFlags = append(logFlags, extraFlags...)

	if c.Name == "__ALL__" && rt.ComposeFile != "" && rt.Bin != "" {
		cmdArgs := append([]string{}, rt.PrefixArgs...)
		cmdArgs = append(cmdArgs, "-f", rt.ComposeFile, "logs")
		cmdArgs = append(cmdArgs, logFlags...)
		execReplace(rt.Bin, append([]string{filepath.Base(rt.Bin)}, cmdArgs...))
	}

	execArgs := append([]string{"docker", "logs"}, logFlags...)
	execArgs = append(execArgs, c.Name)
	execReplace(docker, execArgs)
}

func cmdLifecycle(action string, args []string) {
	includeStopped := action == "start" || action == "restart"
	rt := detectCompose()

	if len(args) == 0 {
		c, _ := resolveContainer("", includeStopped, rt.ComposeFile != "")
		if c.Name == "__ALL__" && rt.ComposeFile != "" && rt.Bin != "" {
			cmdArgs := append([]string{}, rt.PrefixArgs...)
			cmdArgs = append(cmdArgs, "-f", rt.ComposeFile, action)
			runAttached(rt.Bin, cmdArgs)
			return
		}
		runAttached(docker, []string{action, c.Name})
		return
	}

	// If in a Compose project and all args are Compose services, delegate to Compose
	if rt.ComposeFile != "" && rt.Bin != "" {
		services := composeServices(rt)
		svcSet := make(map[string]bool, len(services))
		for _, s := range services {
			svcSet[s] = true
		}
		allCompose := len(args) > 0
		for _, a := range args {
			if !svcSet[a] {
				allCompose = false
				break
			}
		}
		if allCompose {
			cmdArgs := append([]string{}, rt.PrefixArgs...)
			cmdArgs = append(cmdArgs, "-f", rt.ComposeFile, action)
			cmdArgs = append(cmdArgs, args...)
			runAttached(rt.Bin, cmdArgs)
			return
		}
	}

	for _, target := range args {
		if strings.HasPrefix(target, "-") {
			execReplace(docker, append([]string{"docker", action}, args...))
		}
		c, _ := resolveContainer(target, includeStopped, false)
		fmt.Printf("%s%sing %s...%s\n", st.Cyan, capitalize(action), c.Name, st.Reset)
		runAttached(docker, []string{action, c.Name})
	}
}

func capitalize(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

func cmdStats(args []string) {
	var targets []string
	var flags []string
	for _, arg := range args {
		if strings.HasPrefix(arg, "-") {
			flags = append(flags, arg)
		} else {
			c, _ := resolveContainer(arg, false, false)
			targets = append(targets, c.Name)
		}
	}

	cmdArgs := []string{"docker", "stats"}
	cmdArgs = append(cmdArgs, flags...)
	cmdArgs = append(cmdArgs, targets...)
	execReplace(docker, cmdArgs)
}

func cmdEnv(args []string) {
	reveal := false
	targetQuery := ""
	for _, arg := range args {
		switch arg {
		case "--reveal", "--show-secrets":
			reveal = true
		default:
			targetQuery = arg
		}
	}

	c, _ := resolveContainer(targetQuery, true, false)
	out, err := exec.Command(docker, "inspect", "--format", "{{json .Config.Env}}", c.Name).Output()
	if err != nil {
		exitWithErr(err)
	}

	var envs []string
	if err := json.Unmarshal(bytes.TrimSpace(out), &envs); err != nil {
		fatal("failed to parse container environment variables")
	}

	sort.Strings(envs)
	fmt.Printf("%sEnvironment for %s:%s\n", st.Bold, c.Name, st.Reset)
	for _, entry := range envs {
		parts := strings.SplitN(entry, "=", 2)
		key := parts[0]
		val := ""
		if len(parts) == 2 {
			val = parts[1]
		}
		if !reveal && isSensitiveKey(key) && val != "" {
			val = st.Yellow + "********" + st.Reset
		}
		fmt.Printf("  %s%-32s%s = %s\n", st.Cyan, key, st.Reset, val)
	}
	if !reveal {
		fmt.Printf("\n%sTip:%s Sensitive values are masked. Re-run with %s--reveal%s to view plaintext.\n",
			st.Dim, st.Reset, st.Cyan, st.Reset)
	}
}

func isSensitiveKey(key string) bool {
	upper := strings.ToUpper(key)
	markers := []string{"SECRET", "PASSWORD", "PASSWD", "TOKEN", "API_KEY", "PRIVATE_KEY", "CREDENTIAL", "AUTH"}
	for _, m := range markers {
		if strings.Contains(upper, m) {
			return true
		}
	}
	return false
}

func cmdIP(args []string) {
	containers, err := listContainers(false)
	if err != nil {
		exitWithErr(err)
	}
	if len(containers) == 0 {
		fmt.Println("No running containers.")
		return
	}

	filter := ""
	if len(args) > 0 {
		c, _ := resolveContainer(args[0], false, false)
		filter = c.Name
	}

	fmt.Printf("%s%-24s %-20s %-18s %s%s\n", st.Bold, "CONTAINER", "NETWORK", "IP ADDRESS", "PORTS", st.Reset)

	type netInspect struct {
		Name            string `json:"Name"`
		NetworkSettings struct {
			Networks map[string]struct {
				IPAddress string `json:"IPAddress"`
			} `json:"Networks"`
		} `json:"NetworkSettings"`
	}

	for _, c := range containers {
		if filter != "" && c.Name != filter {
			continue
		}
		out, inspErr := exec.Command(docker, "inspect", c.Name).Output()
		if inspErr != nil {
			continue
		}
		var parsed []netInspect
		if json.Unmarshal(out, &parsed) != nil || len(parsed) == 0 {
			continue
		}
		nets := parsed[0].NetworkSettings.Networks
		if len(nets) == 0 {
			fmt.Printf("%-24s %-20s %-18s %s\n", c.Name, "none", "-", formatPorts(c.Ports))
			continue
		}
		first := true
		for netName, info := range nets {
			ip := info.IPAddress
			if ip == "" {
				ip = "-"
			}
			if first {
				fmt.Printf("%-24s %-20s %s%-18s%s %s\n",
					truncate(c.Name, 23), truncate(netName, 19), st.Green, ip, st.Reset, formatPorts(c.Ports))
				first = false
			} else {
				fmt.Printf("%-24s %-20s %s%-18s%s\n", "", truncate(netName, 19), st.Green, ip, st.Reset)
			}
		}
	}
}

func cmdClean(args []string) {
	mode := ""
	autoYes := false
	for _, arg := range args {
		switch arg {
		case "--safe":
			mode = "safe"
		case "--deep", "--all":
			mode = "deep"
		case "-y", "--yes", "-f", "--force":
			autoYes = true
		}
	}

	fmt.Printf("%sCurrent Docker disk usage:%s\n", st.Bold, st.Reset)
	runAttached(docker, []string{"system", "df"})
	fmt.Println()

	if mode == "" {
		if !isInteractive() {
			mode = "safe"
		} else {
			picked, ok := interactivePick("Select cleanup level", []PickOption{
				{Value: "safe", Label: "Safe prune  — Stopped containers, dangling images & build cache"},
				{Value: "deep", Label: "Deep prune  — Everything above + all unused images, networks & volumes"},
			})
			if !ok {
				return
			}
			mode = picked
		}
	}

	if mode == "deep" && !autoYes && isInteractive() {
		if !promptConfirm("Deep prune removes ALL unused images and volumes. Proceed?", false) {
			fmt.Println("Aborted.")
			return
		}
	}

	if mode == "safe" {
		runAttached(docker, []string{"container", "prune", "-f"})
		runAttached(docker, []string{"image", "prune", "-f"})
		runAttached(docker, []string{"builder", "prune", "-f"})
	} else {
		runAttached(docker, []string{"system", "prune", "-a", "--volumes", "-f"})
	}
}

func cmdContext(args []string) {
	if len(args) > 0 {
		if args[0] == "ls" || args[0] == "list" {
			execReplace(docker, []string{"docker", "context", "ls"})
		}
		execReplace(docker, []string{"docker", "context", "use", args[0]})
	}

	out, err := exec.Command(docker, "context", "ls", "--format", `{{.Name}}\t{{.Current}}\t{{.DockerEndpoint}}`).Output()
	if err != nil {
		exitWithErr(err)
	}

	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	var opts []PickOption
	for _, line := range lines {
		cols := strings.Split(line, "\t")
		if len(cols) < 3 {
			continue
		}
		marker := " "
		if cols[1] == "true" {
			marker = "*"
		}
		opts = append(opts, PickOption{
			Value: cols[0],
			Label: fmt.Sprintf("[%s] %-18s %s", marker, cols[0], cols[2]),
		})
	}

	if !isInteractive() {
		execReplace(docker, []string{"docker", "context", "ls"})
	}

	picked, ok := interactivePick("Switch Docker context", opts)
	if !ok {
		return
	}
	runAttached(docker, []string{"context", "use", picked})
}

func cmdComposePassthrough(args []string) {
	rt := detectCompose()
	if rt.Bin == "" {
		fatal("neither 'docker compose' plugin nor 'docker-compose' binary was found")
	}
	cmdArgs := append([]string{filepath.Base(rt.Bin)}, rt.PrefixArgs...)
	if rt.ComposeFile != "" {
		cmdArgs = append(cmdArgs, "-f", rt.ComposeFile)
	}
	cmdArgs = append(cmdArgs, args...)
	execReplace(rt.Bin, cmdArgs)
}

// ============================================================================
// Process Execution Helpers
// ============================================================================

func runAttached(binary string, args []string) {
	cmd := exec.Command(binary, args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		exitWithErr(err)
	}
}

func execReplace(binary string, argv []string) {
	if err := syscall.Exec(binary, argv, os.Environ()); err != nil {
		// Fallback to attached child process if syscall.Exec fails
		cmd := exec.Command(binary, argv[1:]...)
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if runErr := cmd.Run(); runErr != nil {
			exitWithErr(runErr)
		}
		os.Exit(0)
	}
}
