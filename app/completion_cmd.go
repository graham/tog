package app

import (
	"fmt"
	"os"
	"strings"
)

// commands is the list of available commands for completion.
// This must be kept in sync with the switch statement in Run().
var commands = []string{
	"serve",
	"watch",
	"verify",
	"routes",
	"schema",
	"env",
	"testdocs",
	"findqueries",
	"inlinetest",
	"agent-prompt",
	"completion",
	"help",
}

func cmdCompletion(cfg Config, args []string) {
	fs := newFlagSet(cfg.Name, "completion", `Generate shell completion scripts.

Usage:
  `+cfg.Name+` completion bash    # Output bash completion script
  `+cfg.Name+` completion zsh     # Output zsh completion script
  `+cfg.Name+` completion fish    # Output fish completion script

To enable completion:

  # Bash (add to ~/.bashrc)
  eval "$(`+cfg.Name+` completion bash)"

  # Zsh (add to ~/.zshrc)
  eval "$(`+cfg.Name+` completion zsh)"

  # Fish (add to ~/.config/fish/config.fish)
  `+cfg.Name+` completion fish | source`)

	fs.Parse(args)

	remaining := fs.Args()
	if len(remaining) == 0 {
		fs.Usage()
		os.Exit(1)
	}

	shell := remaining[0]
	switch shell {
	case "bash":
		fmt.Print(bashCompletion(cfg.Name))
	case "zsh":
		fmt.Print(zshCompletion(cfg.Name))
	case "fish":
		fmt.Print(fishCompletion(cfg.Name))
	default:
		fmt.Fprintf(os.Stderr, "Unknown shell: %s\nSupported: bash, zsh, fish\n", shell)
		os.Exit(1)
	}
}

func bashCompletion(appName string) string {
	cmdList := strings.Join(commands, " ")
	return fmt.Sprintf(`# Bash completion for %s
# Add to ~/.bashrc: eval "$(%s completion bash)"

_%s_completions() {
    local cur prev commands
    cur="${COMP_WORDS[COMP_CWORD]}"
    prev="${COMP_WORDS[COMP_CWORD-1]}"
    commands="%s"

    # Complete commands as first argument
    if [[ ${COMP_CWORD} -eq 1 ]]; then
        COMPREPLY=($(compgen -W "${commands}" -- "${cur}"))
        return 0
    fi

    # Command-specific completions
    case "${prev}" in
        inlinetest)
            # Complete flags for inlinetest
            local flags="--with-user --with-session --with-api-key --admin -X -d -f -i -r -s -S -p --init -w --db -q -v --migrations"
            COMPREPLY=($(compgen -W "${flags}" -- "${cur}"))
            return 0
            ;;
        -f)
            # File completion
            COMPREPLY=($(compgen -f -- "${cur}"))
            return 0
            ;;
        -X)
            # HTTP methods
            COMPREPLY=($(compgen -W "GET POST PUT DELETE PATCH" -- "${cur}"))
            return 0
            ;;
        completion)
            COMPREPLY=($(compgen -W "bash zsh fish" -- "${cur}"))
            return 0
            ;;
        agent-prompt)
            COMPREPLY=($(compgen -W "%s --all" -- "${cur}"))
            return 0
            ;;
        routes)
            COMPREPLY=($(compgen -W "-all -md" -- "${cur}"))
            return 0
            ;;
        testdocs)
            COMPREPLY=($(compgen -W "-o -pkg -root -title" -- "${cur}"))
            return 0
            ;;
        findqueries)
            COMPREPLY=($(compgen -W "-exclude-tests -show-sql" -- "${cur}"))
            return 0
            ;;
    esac

    # Default to file completion
    COMPREPLY=($(compgen -f -- "${cur}"))
}

complete -F _%s_completions %s
`, appName, appName, appName, cmdList, cmdList, appName, appName)
}

func zshCompletion(appName string) string {
	cmdList := strings.Join(commands, " ")
	return fmt.Sprintf(`#compdef %s
# Zsh completion for %s
# Add to ~/.zshrc: eval "$(%s completion zsh)"

_%s() {
    local -a commands
    commands=(
        'serve:Run the HTTP server'
        'watch:Run with auto-reload on file changes'
        'verify:Verify all queries against the database'
        'routes:List all registered routes'
        'schema:Dump database schema as JSON'
        'env:Show environment variables and database configuration'
        'testdocs:Generate test documentation HTML'
        'findqueries:Find unregistered SQL queries'
        'inlinetest:Execute requests against in-memory test database'
        'agent-prompt:Show AI-optimized documentation for commands'
        'completion:Generate shell completion scripts'
        'help:Show help'
    )

    local -a inlinetest_flags
    inlinetest_flags=(
        '--with-user=[Create user with email]:email:'
        '--with-session[Create session for user]'
        '--with-api-key[Create API key for user]'
        '--admin[Make user an admin]'
        '-X[HTTP method]:method:(GET POST PUT DELETE PATCH)'
        '-d[Request body JSON]:body:'
        '-f[Read commands from file]:file:_files'
        '-i[Read URLs from stdin]'
        '-r[Enable readline support]'
        '-s[Persist cookies across requests]'
        '-S[Disable session persistence]'
        '-p[Persist session to file]:file:_files'
        '--init[Run commands from file before interactive mode]:file:_files'
        '-w[Watch for file changes and restart]'
        '--db[Use real database from DATABASE_CONFIG]'
        '-q[Quiet mode - only print failures]'
        '-v[Verbose output]'
        '--migrations=[Migrations directory]:dir:_files -/'
    )

    _arguments -C \
        '1:command:->command' \
        '*::arg:->args'

    case "$state" in
        command)
            _describe -t commands 'commands' commands
            ;;
        args)
            case "${words[1]}" in
                inlinetest)
                    _arguments $inlinetest_flags
                    ;;
                completion)
                    _values 'shell' bash zsh fish
                    ;;
                agent-prompt)
                    _values 'command' %s --all
                    ;;
                routes)
                    _arguments '-all[Show all routes]' '-md[Output as markdown]'
                    ;;
                testdocs)
                    _arguments \
                        '-o[Output file]:file:_files' \
                        '-pkg[Package pattern]:pattern:' \
                        '-root[Root directory]:dir:_files -/' \
                        '-title[Page title]:title:'
                    ;;
                findqueries)
                    _arguments \
                        '-exclude-tests[Exclude test files]' \
                        '-show-sql[Show SQL in output]'
                    ;;
                *)
                    _files
                    ;;
            esac
            ;;
    esac
}

compdef _%s %s
`, appName, appName, appName, appName, cmdList, appName, appName)
}

func fishCompletion(appName string) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf(`# Fish completion for %s
# Add to ~/.config/fish/config.fish: %s completion fish | source

# Disable file completion by default
complete -c %s -f

# Commands
`, appName, appName, appName))

	// Add each command with description
	cmdDescs := map[string]string{
		"serve":        "Run the HTTP server",
		"watch":        "Run with auto-reload on file changes",
		"verify":       "Verify all queries against the database",
		"routes":       "List all registered routes",
		"schema":       "Dump database schema as JSON",
		"env":          "Show environment variables and database configuration",
		"testdocs":     "Generate test documentation HTML",
		"findqueries":  "Find unregistered SQL queries",
		"inlinetest":   "Execute requests against in-memory test database",
		"agent-prompt": "Show AI-optimized documentation for commands",
		"completion":   "Generate shell completion scripts",
		"help":         "Show help",
	}

	for _, cmd := range commands {
		desc := cmdDescs[cmd]
		sb.WriteString(fmt.Sprintf("complete -c %s -n '__fish_use_subcommand' -a '%s' -d '%s'\n",
			appName, cmd, desc))
	}

	// inlinetest flags
	sb.WriteString(fmt.Sprintf(`
# inlinetest flags
complete -c %s -n '__fish_seen_subcommand_from inlinetest' -l with-user -d 'Create user with email' -r
complete -c %s -n '__fish_seen_subcommand_from inlinetest' -l with-session -d 'Create session for user'
complete -c %s -n '__fish_seen_subcommand_from inlinetest' -l with-api-key -d 'Create API key for user'
complete -c %s -n '__fish_seen_subcommand_from inlinetest' -l admin -d 'Make user an admin'
complete -c %s -n '__fish_seen_subcommand_from inlinetest' -s X -d 'HTTP method' -a 'GET POST PUT DELETE PATCH'
complete -c %s -n '__fish_seen_subcommand_from inlinetest' -s d -d 'Request body JSON' -r
complete -c %s -n '__fish_seen_subcommand_from inlinetest' -s f -d 'Read commands from file' -r -F
complete -c %s -n '__fish_seen_subcommand_from inlinetest' -s i -d 'Read URLs from stdin'
complete -c %s -n '__fish_seen_subcommand_from inlinetest' -s r -d 'Enable readline support'
complete -c %s -n '__fish_seen_subcommand_from inlinetest' -s s -d 'Persist cookies across requests'
complete -c %s -n '__fish_seen_subcommand_from inlinetest' -s S -d 'Disable session persistence'
complete -c %s -n '__fish_seen_subcommand_from inlinetest' -s p -d 'Persist session to file' -r -F
complete -c %s -n '__fish_seen_subcommand_from inlinetest' -l init -d 'Run commands from file before interactive mode' -r -F
complete -c %s -n '__fish_seen_subcommand_from inlinetest' -s w -d 'Watch for file changes and restart'
complete -c %s -n '__fish_seen_subcommand_from inlinetest' -l db -d 'Use real database from DATABASE_CONFIG'
complete -c %s -n '__fish_seen_subcommand_from inlinetest' -s q -d 'Quiet mode - only print failures'
complete -c %s -n '__fish_seen_subcommand_from inlinetest' -s v -d 'Verbose output'
complete -c %s -n '__fish_seen_subcommand_from inlinetest' -l migrations -d 'Migrations directory' -r -a '(__fish_complete_directories)'

# completion subcommands
complete -c %s -n '__fish_seen_subcommand_from completion' -a 'bash zsh fish'

# agent-prompt subcommands
complete -c %s -n '__fish_seen_subcommand_from agent-prompt' -a '%s --all'

# routes flags
complete -c %s -n '__fish_seen_subcommand_from routes' -s all -d 'Show all routes'
complete -c %s -n '__fish_seen_subcommand_from routes' -s md -d 'Output as markdown'

# testdocs flags
complete -c %s -n '__fish_seen_subcommand_from testdocs' -s o -d 'Output file' -r -F
complete -c %s -n '__fish_seen_subcommand_from testdocs' -s pkg -d 'Package pattern' -r
complete -c %s -n '__fish_seen_subcommand_from testdocs' -s root -d 'Root directory' -r -a '(__fish_complete_directories)'
complete -c %s -n '__fish_seen_subcommand_from testdocs' -s title -d 'Page title' -r

# findqueries flags
complete -c %s -n '__fish_seen_subcommand_from findqueries' -l exclude-tests -d 'Exclude test files'
complete -c %s -n '__fish_seen_subcommand_from findqueries' -l show-sql -d 'Show SQL in output'
`, appName, appName, appName, appName, appName, appName, appName, appName, appName, appName, appName, appName, appName, appName, appName, appName, appName, appName,
		appName, appName, strings.Join(commands, " "),
		appName, appName, appName, appName, appName, appName, appName, appName))

	return sb.String()
}
