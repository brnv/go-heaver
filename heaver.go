package heaver

import (
	"encoding/json"
	"errors"
	"os/exec"
	"regexp"
	"strings"

	"github.com/brnv/go-lxc"
)

var (
	createArgs       = []string{"heaver", "-CSn", ""}
	netInterfaceArgs = []string{"--net", "br0"}
	controlArgs      = []string{"heaver", "", ""}
	startArg         = "-Sn"
	stopArg          = "-Tn"
	destroyArg       = "-Dn"
	reIp             = regexp.MustCompile("(\\d{1,3}\\.\\d{1,3}\\.\\d{1,3}\\.\\d{1,3})")
	reStarted        = regexp.MustCompile("started")
	reStopped        = regexp.MustCompile("stopped")
	reDestroyed      = regexp.MustCompile("destroyed")
	reList           = regexp.MustCompile(`\s*([\d\w-\.]*):\s([a-z]*).*:\s([\d\.]*)/`)
)

type Image struct {
	Updated string `json:"updated"`
	Size    int64  `json:"size"`
	ZfsPath string `json:"zfs_path"`
}

func Create(containerName string, image []string, key string) (lxc.Container, error) {
	createArgs[2] = containerName

	args := createArgs
	for _, i := range image {
		args = append(args, "-i")
		args = append(args, i)
	}
	if key != "" {
		args = append(args, "--raw-key")
		args = append(args, key)
	}
	for _, n := range netInterfaceArgs {
		args = append(args, n)
	}

	cmd := getHeaverCmd(args)

	output, err := cmd.Output()
	if err != nil {
		return lxc.Container{Status: "error"}, errors.New(string(output))
	}

	ip := ""
	matches := reIp.FindStringSubmatch(string(output))
	if matches != nil {
		ip = matches[1]
	}

	container := lxc.Container{
		Name:   containerName,
		Status: "created",
		Image:  image,
		Ip:     ip,
	}

	return container, nil
}

func Control(containerName string, action string) error {
	var reControl *regexp.Regexp
	switch action {
	case "start":
		controlArgs[1] = startArg
		reControl = reStarted
	case "stop":
		controlArgs[1] = stopArg
		reControl = reStopped
	case "destroy":
		controlArgs[1] = destroyArg
		reControl = reDestroyed
	}
	controlArgs[2] = containerName

	answer, err := getHeaverCmd(controlArgs).Output()
	if err != nil {
		return errors.New(string(answer))
	}

	matches := reControl.FindStringSubmatch(string(answer))
	if matches == nil {
		return errors.New("Can't perform " + action)
	}

	return nil
}

func ListContainers(host string) (map[string]lxc.Container, error) {
	cmd := exec.Command("heaver", "-L")
	output, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	list := make(map[string]lxc.Container)
	containers := strings.Split(string(output), "\n")
	for _, container := range containers {
		parsed := reList.FindStringSubmatch(container)
		if parsed != nil {
			name := parsed[1]
			list[name] = lxc.Container{
				Name:   name,
				Host:   host,
				Status: parsed[2],
				Ip:     parsed[3],
			}
		}
	}

	return list, nil
}

func ListImages() (map[string]Image, error) {
	cmd := exec.Command("heaver-img", "-Qj")
	raw, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	jsonResp := struct {
		Status string          `json:"status"`
		Data   json.RawMessage `json:"data"`
		Error  string          `json:"error"`
	}{}

	err = json.Unmarshal(raw, &jsonResp)
	if err != nil {
		return nil, err
	}

	if jsonResp.Error != "" {
		return nil, errors.New(jsonResp.Error)
	}

	list := make(map[string]Image)
	err = json.Unmarshal(jsonResp.Data, &list)
	if err != nil {
		return nil, err
	}

	return list, nil
}

func getHeaverCmd(args []string) *exec.Cmd {
	cmd := &exec.Cmd{
		Path: "/usr/bin/heaver",
		Args: args,
	}
	return cmd
}
