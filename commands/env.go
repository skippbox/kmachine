package commands

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"text/template"

	"github.com/docker/machine/cli"
	"github.com/docker/machine/libmachine/log"
)

const (
	envTmpl = `kubectl config set-cluster kmachine --server={{ .K8sHost }} --insecure-skip-tls-verify=true{{ .Suffix2 }}kubectl config set-credentials kuser --token=abcdefghijkl{{ .Suffix2 }}kubectl config set-context kmachine --user=kuser --cluster=kmachine{{ .Suffix2 }}kubectl config use-context kmachine{{ .Suffix2 }}{{ .Prefix }}DOCKER_TLS_VERIFY{{ .Delimiter }}{{ .DockerTLSVerify }}{{ .Suffix }}{{ .Prefix }}DOCKER_HOST{{ .Delimiter }}{{ .DockerHost }}{{ .Suffix }}{{ .Prefix }}DOCKER_CERT_PATH{{ .Delimiter }}{{ .DockerCertPath }}{{ .Suffix }}{{ .Prefix }}DOCKER_MACHINE_NAME{{ .Delimiter }}{{ .MachineName }}{{ .Suffix }}{{ if .NoProxyVar }}{{ .Prefix }}{{ .NoProxyVar }}{{ .Delimiter }}{{ .NoProxyValue }}{{ .Suffix }}{{end}}{{ .UsageHint }}`
)

var (
	improperEnvArgsError = errors.New("Error: Expected either one machine name, or -u flag to unset the variables in the arguments.")
)

type ShellConfig struct {
	Prefix          string
	Delimiter       string
	Suffix          string
	Suffix2			string
	DockerCertPath  string
	DockerHost      string
	K8sHost			string
	DockerTLSVerify string
	UsageHint       string
	MachineName     string
	NoProxyVar      string
	NoProxyValue    string
}

func cmdEnv(c *cli.Context) {
	// Ensure that log messages always go to stderr when this command is
	// being run (it is intended to be run in a subshell)
	log.SetOutWriter(os.Stderr)

	if len(c.Args()) != 1 && !c.Bool("unset") {
		fatal(improperEnvArgsError)
	}

	h := getFirstArgHost(c)

	dockerHost, authOptions, err := runConnectionBoilerplate(h, c)
	if err != nil {
		fatalf("Error running connection boilerplate: %s", err)
	}

	mParts := strings.Split(dockerHost, "://")
	mParts = strings.Split(mParts[1], ":")
	k8sHost := fmt.Sprintf("https://%s:6443", mParts[0])

	userShell := c.String("shell")
	if userShell == "" {
		shell, err := detectShell()
		if err != nil {
			fatal(err)
		}
		userShell = shell
	}

	t := template.New("envConfig")

	usageHint := generateUsageHint(c.App.Name, c.Args().First(), userShell)

	shellCfg := &ShellConfig{
		DockerCertPath:  authOptions.CertDir,
		DockerHost:      dockerHost,
        K8sHost:         k8sHost,
		DockerTLSVerify: "1",
		UsageHint:       usageHint,
		MachineName:     h.Name,
	}

	if c.Bool("no-proxy") {
		ip, err := h.Driver.GetIP()
		if err != nil {
			fatalf("Error getting host IP: %s", err)
		}

		// first check for an existing lower case no_proxy var
		noProxyVar := "no_proxy"
		noProxyValue := os.Getenv("no_proxy")

		// otherwise default to allcaps HTTP_PROXY
		if noProxyValue == "" {
			noProxyVar = "NO_PROXY"
			noProxyValue = os.Getenv("NO_PROXY")
		}

		// add the docker host to the no_proxy list idempotently
		switch {
		case noProxyValue == "":
			noProxyValue = ip
		case strings.Contains(noProxyValue, ip):
			//ip already in no_proxy list, nothing to do
		default:
			noProxyValue = fmt.Sprintf("%s,%s", noProxyValue, ip)
		}

		shellCfg.NoProxyVar = noProxyVar
		shellCfg.NoProxyValue = noProxyValue
	}

	// unset vars
	if c.Bool("unset") {
		switch userShell {
		case "fish":
			shellCfg.Prefix = "set -e "
			shellCfg.Delimiter = ""
			shellCfg.Suffix = ";\n"
		case "powershell":
			shellCfg.Prefix = "Remove-Item Env:\\\\"
			shellCfg.Delimiter = ""
			shellCfg.Suffix = "\n"
		case "cmd":
			// since there is no way to unset vars in cmd just reset to empty
			shellCfg.DockerCertPath = ""
			shellCfg.DockerHost = ""
			shellCfg.DockerTLSVerify = ""
			shellCfg.Prefix = "set "
			shellCfg.Delimiter = "="
			shellCfg.Suffix = "\n"
		default:
			shellCfg.Prefix = "unset "
			shellCfg.Delimiter = " "
			shellCfg.Suffix = "\n"
		}

		tmpl, err := t.Parse(envTmpl)
		if err != nil {
			fatal(err)
		}

		if err := tmpl.Execute(os.Stdout, shellCfg); err != nil {
			fatal(err)
		}
		return
	}

	switch userShell {
	case "fish":
		shellCfg.Prefix = "set -x "
		shellCfg.Suffix = "\";\n"
		shellCfg.Delimiter = " \""
	case "powershell":
		shellCfg.Prefix = "$Env:"
		shellCfg.Suffix = "\"\n"
		shellCfg.Delimiter = " = \""
	case "cmd":
		shellCfg.Prefix = "set "
		shellCfg.Suffix = "\n"
		shellCfg.Delimiter = "="
	default:
		shellCfg.Prefix = "export "
		shellCfg.Suffix = "\"\n"
		shellCfg.Suffix2 = "\n"
		shellCfg.Delimiter = "=\""
	}

	tmpl, err := t.Parse(envTmpl)
	if err != nil {
		fatal(err)
	}

	if err := tmpl.Execute(os.Stdout, shellCfg); err != nil {
		fatal(err)
	}
}

func generateUsageHint(appName, machineName, userShell string) string {
	cmd := ""
	switch userShell {
	case "fish":
		if machineName != "" {
			cmd = fmt.Sprintf("eval (%s env %s)", appName, machineName)
		} else {
			cmd = fmt.Sprintf("eval (%s env)", appName)
		}
	case "powershell":
		if machineName != "" {
			cmd = fmt.Sprintf("%s env --shell=powershell %s | Invoke-Expression", appName, machineName)
		} else {
			cmd = fmt.Sprintf("%s env --shell=powershell | Invoke-Expression", appName)
		}
	case "cmd":
		cmd = "copy and paste the above values into your command prompt"
	default:
		if machineName != "" {
			cmd = fmt.Sprintf("eval \"$(%s env %s)\"", appName, machineName)
		} else {
			cmd = fmt.Sprintf("eval \"$(%s env)\"", appName)
		}
	}

	return fmt.Sprintf("# Run this command to configure your shell: \n# %s\n", cmd)
}
