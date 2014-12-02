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
	createArgs       = []string{"heaver", "-CSn", "", "--format", "json"}
	netInterfaceArgs = []string{"--net", "br0"}
	controlArgs      = []string{"heaver", "", ""}
	listArgs         = []string{"heaver", "-Lm", "--format", "json"}
	startArg         = "-Sn"
	stopArg          = "-Tn"
	destroyArg       = "-Dn"
	reStarted        = regexp.MustCompile("started")
	reStopped        = regexp.MustCompile("stopped")
	reDestroyed      = regexp.MustCompile("destroyed")
)

const (
	StatusActive   = "active"
	StatusInactive = "inactive"
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
	messages := strings.Split(string(output), "\n")

	heaverOutputJson := struct {
		Data struct {
			Ips        []string
			Filesystem string
			Mountpoint string
		}
	}{}

	err = json.Unmarshal([]byte(messages[0]), &heaverOutputJson)

	if err != nil {
		return lxc.Container{Status: "error"}, err
	}

	ip := ""
	if len(heaverOutputJson.Data.Ips) != 0 {
		ip = heaverOutputJson.Data.Ips[0]
	}

	container := lxc.Container{
		Name:       containerName,
		Status:     "created",
		Image:      image,
		Filesystem: heaverOutputJson.Data.Filesystem,
		Mountpoint: heaverOutputJson.Data.Mountpoint,
		Ip:         ip,
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
	cmd := getHeaverCmd(listArgs)
	output, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	heaverOutputJson := struct {
		Data map[string]struct {
			Active     bool
			Ips        map[string][]string
			Filesystem string
			Mountpoint string
		}
	}{}

	err = json.Unmarshal(output, &heaverOutputJson)

	list := map[string]lxc.Container{}

	for name, container := range heaverOutputJson.Data {
		status := StatusActive
		if container.Active == false {
			status = StatusInactive
		}

		list[name] = lxc.Container{
			Name:       name,
			Host:       host,
			Status:     status,
			Ips:        container.Ips,
			Filesystem: container.Filesystem,
			Mountpoint: container.Mountpoint,
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
