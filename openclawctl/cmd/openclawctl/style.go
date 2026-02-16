package main

import (
	"fmt"
	"os"
	"strings"
)

const (
	ansiReset = "\x1b[0m"
	ansiBold  = "\x1b[1m"
	ansiDim   = "\x1b[2m"

	ansiFgBlack       = "\x1b[30m"
	ansiFgRed         = "\x1b[31m"
	ansiFgGreen       = "\x1b[32m"
	ansiFgYellow      = "\x1b[33m"
	ansiFgBlue        = "\x1b[34m"
	ansiFgMagenta     = "\x1b[35m"
	ansiFgCyan        = "\x1b[36m"
	ansiFgWhite       = "\x1b[37m"
	ansiFgBrightBlack = "\x1b[90m"
	ansiFgBrightWhite = "\x1b[97m"
	ansiBgRed         = "\x1b[41m"
	ansiBgGreen       = "\x1b[42m"
	ansiBgYellow      = "\x1b[43m"
	ansiBgBlue        = "\x1b[44m"
	ansiBgBrightBlack = "\x1b[100m"
	ansiBgBrightWhite = "\x1b[107m"
)

type colorPrinter struct {
	stdout bool
	stderr bool
}

var printer = colorPrinter{
	stdout: shouldUseColor(os.Stdout),
	stderr: shouldUseColor(os.Stderr),
}

func shouldUseColor(stream *os.File) bool {
	if noColor := strings.TrimSpace(os.Getenv("NO_COLOR")); noColor != "" {
		return false
	}
	if strings.TrimSpace(os.Getenv("CLICOLOR")) == "0" {
		return false
	}
	if force := strings.TrimSpace(os.Getenv("CLICOLOR_FORCE")); force != "" && force != "0" {
		return true
	}
	if term := strings.ToLower(strings.TrimSpace(os.Getenv("TERM"))); term == "dumb" {
		return false
	}
	if stream == nil {
		return false
	}
	info, err := stream.Stat()
	if err != nil {
		return false
	}
	return (info.Mode() & os.ModeCharDevice) != 0
}

func paint(enabled bool, text string, codes ...string) string {
	if !enabled || text == "" {
		return text
	}
	var b strings.Builder
	for _, code := range codes {
		b.WriteString(code)
	}
	b.WriteString(text)
	b.WriteString(ansiReset)
	return b.String()
}

func accent(text string) string {
	return paint(printer.stdout, text, ansiBold, ansiFgCyan)
}

func keyLabel(text string) string {
	return paint(printer.stdout, text, ansiDim, ansiFgBrightBlack)
}

func badge(enabled bool, text, bg, fg string) string {
	if !enabled {
		return strings.TrimSpace(text)
	}
	content := " " + strings.ToUpper(strings.TrimSpace(text)) + " "
	return paint(enabled, content, ansiBold, bg, fg)
}

func badgeInfo(text string) string {
	return badge(printer.stdout, text, ansiBgBlue, ansiFgBrightWhite)
}

func badgeSuccess(text string) string {
	return badge(printer.stdout, text, ansiBgGreen, ansiFgBlack)
}

func badgeWarn(text string) string {
	return badge(printer.stderr, text, ansiBgYellow, ansiFgBlack)
}

func badgeError(text string) string {
	return badge(printer.stderr, text, ansiBgRed, ansiFgBrightWhite)
}

func outInfof(format string, args ...any) {
	fmt.Fprintf(os.Stdout, "%s %s\n", badgeInfo("info"), fmt.Sprintf(format, args...))
}

func outSuccessf(format string, args ...any) {
	fmt.Fprintf(os.Stdout, "%s %s\n", badgeSuccess("ok"), fmt.Sprintf(format, args...))
}

func outWarnf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "%s %s\n", badgeWarn("warn"), fmt.Sprintf(format, args...))
}

func outErrorf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "%s %s\n", badgeError("error"), fmt.Sprintf(format, args...))
}

func stateBadge(value string) string {
	normalized := strings.ToLower(strings.TrimSpace(value))
	switch {
	case normalized == "":
		return badge(printer.stdout, "unknown", ansiBgBrightBlack, ansiFgBrightWhite)
	case strings.Contains(normalized, "healthy"), strings.Contains(normalized, "reachable"), normalized == "running":
		return badge(printer.stdout, value, ansiBgGreen, ansiFgBlack)
	case strings.Contains(normalized, "unhealthy"), strings.Contains(normalized, "unreachable"), strings.Contains(normalized, "error"), normalized == "dead", normalized == "exited", normalized == "false", normalized == "not-found":
		return badge(printer.stdout, value, ansiBgRed, ansiFgBrightWhite)
	default:
		return badge(printer.stdout, value, ansiBgYellow, ansiFgBlack)
	}
}

func boolBadge(v bool) string {
	if v {
		return badge(printer.stdout, "true", ansiBgGreen, ansiFgBlack)
	}
	return badge(printer.stdout, "false", ansiBgRed, ansiFgBrightWhite)
}

func colorizeDiff(diffText string) string {
	if !printer.stdout {
		return diffText
	}

	lines := strings.Split(diffText, "\n")
	for i, line := range lines {
		switch {
		case strings.HasPrefix(line, "+++ "), strings.HasPrefix(line, "--- "):
			lines[i] = paint(printer.stdout, line, ansiBold, ansiFgCyan)
		case strings.HasPrefix(line, "@@"):
			lines[i] = paint(printer.stdout, line, ansiBold, ansiFgMagenta)
		case strings.HasPrefix(line, "+"):
			if strings.HasPrefix(line, "+++") {
				continue
			}
			lines[i] = paint(printer.stdout, line, ansiBgGreen, ansiFgBlack)
		case strings.HasPrefix(line, "-"):
			if strings.HasPrefix(line, "---") {
				continue
			}
			lines[i] = paint(printer.stdout, line, ansiBgRed, ansiFgBrightWhite)
		}
	}
	return strings.Join(lines, "\n")
}
