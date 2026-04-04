set -gx TMPDIR "$HOME/.tmp"
set -g fish_greeting ""

if not test -d "$TMPDIR"
    mkdir -p "$TMPDIR"
end

if not set -q COLORTERM
    set -gx COLORTERM truecolor
end

fish_add_path "$HOME/.local/bin"
fish_add_path "$HOME/.termux/bin"

function clear
    command clear
    if type -q tput
        set -l lines (tput lines 2>/dev/null)
        if test -n "$lines"; and test "$lines" -gt 5
            set -l padding (math "$lines - 3")
            printf (string repeat -n $padding "\n")
        end
    end
    commandline -f repaint
end

if type -q eza
    alias ls='eza --color=always --group-directories-first --icons'
    alias la='eza --color=always --group-directories-first --icons -a'
    alias ll='eza --color=always --group-directories-first --icons -al'
    alias tree='eza -aT --icons --color=always --group-directories-first'
else
    alias ls='ls --color=auto'
    alias la='ls -a --color=auto'
    alias ll='ls -al --color=auto'
end

if type -q zoxide
    set -l __tooie_zoxide_init (zoxide init fish --cmd cd 2>/dev/null)
    if test $status -eq 0; and test -n "$__tooie_zoxide_init"
        echo "$__tooie_zoxide_init" | source
    end

    if functions -q _zoxide_cd
        functions -e _zoxide_cd
    end
    functions --copy cd _zoxide_cd

    function cd --wraps=_zoxide_cd
        _zoxide_cd $argv
        and ls
    end
else
    function cd
        builtin cd $argv
        and ls
    end
end

if status is-interactive
    and not set -q TMUX
    and type -q tmux
    and not set -q SSH_CONNECTION
    and not set -q SSH_CLIENT
    and not set -q SSH_TTY
    set -l __tooie_base "main"
    if set -q TOOIE_TMUX_SESSION_BASE
        set __tooie_base (string trim -- "$TOOIE_TMUX_SESSION_BASE")
    else if set -q TOOIE_TMUX_PURPOSE
        set __tooie_base (string trim -- "$TOOIE_TMUX_PURPOSE")
    end
    if test -z "$__tooie_base"
        set __tooie_base "main"
    end
    set __tooie_base (string lower -- "$__tooie_base")
    set __tooie_base (string replace -ra '[^a-z0-9_-]+' '-' -- "$__tooie_base")
    if test -z "$__tooie_base"
        set __tooie_base "main"
    end

    set -l __tooie_name "$__tooie_base"
    set -l __tooie_idx 2
    while tmux has-session -t "$__tooie_name" >/dev/null 2>&1
        set __tooie_name "$__tooie_base-$__tooie_idx"
        set __tooie_idx (math "$__tooie_idx + 1")
    end
    exec tmux new-session -s "$__tooie_name" 2>/dev/null
end

if status is-interactive
    if test "$TOOIE_STARSHIP_MODE" != "off"; and type -q starship
        set -l __tooie_starship_init (starship init fish 2>/dev/null)
        if test $status -eq 0; and test -n "$__tooie_starship_init"
            echo "$__tooie_starship_init" | source
        end
    end
    clear
end
