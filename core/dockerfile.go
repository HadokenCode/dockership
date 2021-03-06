package core

import (
	"bytes"
	"fmt"
	"regexp"
)

type Dockerfile struct {
	content     []byte
	project     *Project
	revision    Revision
	environment *Environment
	Files       []*File
}

type File struct {
	Name    string
	Content []byte
}

func NewDockerfile(content []byte, p *Project, r Revision, e *Environment) *Dockerfile {
	return &Dockerfile{
		content:     content,
		project:     p,
		revision:    r,
		environment: e,
	}
}

func (d *Dockerfile) Get() []byte {
	result := d.content
	result = d.resolveInfoVariables(result)
	result = d.resolveEtcdVariables(result)

	return result
}

func (d *Dockerfile) resolveInfoVariables(result []byte) []byte {
	if d.project == nil || d.environment == nil {
		return result
	}

	vars := map[string]string{
		"PROJECT": d.project.Name,
		"ENV":     d.environment.Name,
		"VCS":     string(d.project.Repository),
		"REV":     d.revision.GetShort(),
		"HOST":    d.environment.Host,
	}

	for name, value := range vars {
		varName := []byte(fmt.Sprintf("$DOCKERSHIP_%s", name))
		result = bytes.Replace(result, varName, []byte(value), -1)
	}

	return result
}

var etcdVars = regexp.MustCompile("\\$ETCD_([A-Za-z_]*)")

func (d *Dockerfile) resolveEtcdVariables(result []byte) []byte {
	if d.environment == nil || d.environment.EtcdServers == nil {
		return result
	}

	etcd, err := NewEtcd(d.environment.EtcdServers)
	if err != nil {
		Warning("Unable to connect to etcd", "err", err, "environment", d.environment.Name)
	}

	for _, m := range etcdVars.FindAllSubmatch(result, -1) {
		val, err := d.getEtcdValue(etcd, m[1])
		if err == nil {
			result = bytes.Replace(result, m[0], val, -1)
		}
	}

	return result
}

func (d *Dockerfile) getEtcdValue(etcd *Etcd, key []byte) ([]byte, error) {
	etcdKey := string(bytes.Replace(key, []byte("__"), []byte("/"), -1))
	value, err := etcd.Get(etcdKey)
	if err != nil {
		Warning("Unable to retrieve key from etcd", "key", etcdKey, "environment", d.environment.Name)
		return []byte(""), err
	}

	return []byte(value), nil
}
