/*
Copyright 2021 The KubeVela Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

   http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/c-bata/go-prompt"
	"github.com/c-bata/go-prompt/completer"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/oam-dev/kubevela/apis/types"
	"github.com/oam-dev/kubevela/pkg/cue"
	"github.com/oam-dev/kubevela/pkg/cue/model/value"
	"github.com/oam-dev/kubevela/pkg/oam"
	"github.com/oam-dev/kubevela/pkg/utils/common"
	"github.com/oam-dev/kubevela/pkg/utils/util"
	"github.com/oam-dev/kubevela/pkg/velaql"
	querytypes "github.com/oam-dev/kubevela/pkg/velaql/providers/query/types"
)

var k8sClient client.Client

// Filter filter options
type Filter struct {
	Component        string
	Cluster          string
	ClusterNamespace string
}

// NewQlCommand creates `ql` command for executing velaQL
func NewQlCommand(c common.Args, order string, ioStreams util.IOStreams) *cobra.Command {
	ctx := context.Background()
	cmd := &cobra.Command{
		Use:     "ql",
		Short:   "Show result of executing velaQL.",
		Long:    "Show result of executing velaQL.",
		Example: `vela ql "view{parameter=value1,parameter=value2}"`,
		RunE: func(cmd *cobra.Command, args []string) error {
			argsLength := len(args)
			if argsLength == 0 {
				NewKubeVelaPrompt(ctx)
				// return fmt.Errorf("please specify an VelaQL statement")
			}
			velaQL := args[0]
			newClient, err := c.GetClient()
			if err != nil {
				return errors.Wrapf(err, "failed to get client %s", velaQL)
			}
			return printVelaQLResult(ctx, newClient, c, velaQL, cmd)
		},
		Annotations: map[string]string{
			types.TagCommandOrder: order,
			types.TagCommandType:  types.TypeApp,
		},
	}
	cmd.SetOut(ioStreams.Out)
	return cmd
}

// printVelaQLResult show velaQL result
func printVelaQLResult(ctx context.Context, client client.Client, velaC common.Args, velaQL string, cmd *cobra.Command) error {
	queryValue, err := QueryValue(ctx, client, velaC, velaQL)
	if err != nil {
		return err
	}
	response, err := queryValue.CueValue().MarshalJSON()
	if err != nil {
		return err
	}
	var out bytes.Buffer
	err = json.Indent(&out, response, "", "    ")
	if err != nil {
		return err
	}
	cmd.Printf("%s\n", out.String())
	return nil
}

// MakeVelaQL build velaQL
func MakeVelaQL(view string, params map[string]string, action string) string {
	var paramString string
	for key, value := range params {
		if paramString != "" {
			paramString = fmt.Sprintf("%s, %s=%s", paramString, key, value)
		} else {
			paramString = fmt.Sprintf("%s=%s", key, value)
		}
	}
	return fmt.Sprintf("%s{%s}.%s", view, paramString, action)
}

// GetServiceEndpoints get service endpoints by velaQL
func GetServiceEndpoints(ctx context.Context, client client.Client, appName string, namespace string, velaC common.Args, f Filter) ([]querytypes.ServiceEndpoint, error) {
	params := map[string]string{
		"appName": appName,
		"appNs":   namespace,
	}
	if f.Component != "" {
		params["name"] = f.Component
	}
	if f.Cluster != "" && f.ClusterNamespace != "" {
		params["cluster"] = f.Cluster
		params["clusterNs"] = f.ClusterNamespace
	}

	velaQL := MakeVelaQL("service-endpoints-view", params, "status")
	queryValue, err := QueryValue(ctx, client, velaC, velaQL)
	if err != nil {
		return nil, err
	}
	var response = struct {
		Endpoints []querytypes.ServiceEndpoint `json:"endpoints"`
		Error     string                       `json:"error"`
	}{}
	if err := queryValue.CueValue().Decode(&response); err != nil {
		return nil, err
	}
	if response.Error != "" {
		return nil, fmt.Errorf(response.Error)
	}
	return response.Endpoints, nil
}

// QueryValue get queryValue from velaQL
func QueryValue(ctx context.Context, client client.Client, velaC common.Args, velaQL string) (*value.Value, error) {
	dm, err := velaC.GetDiscoveryMapper()
	if err != nil {
		return nil, err
	}
	pd, err := velaC.GetPackageDiscover()
	if err != nil {
		return nil, err
	}
	queryView, err := velaql.ParseVelaQL(velaQL)
	if err != nil {
		return nil, err
	}
	config, err := velaC.GetConfig()
	if err != nil {
		return nil, err
	}
	queryValue, err := velaql.NewViewHandler(client, config, dm, pd).QueryView(ctx, queryView)
	if err != nil {
		return nil, err
	}
	return queryValue, nil
}

// NewKubeVelaPrompt new prompt
func NewKubeVelaPrompt(ctx context.Context) {
	c, err := NewCompleter(ctx)
	if err != nil {
		os.Exit(1)
	}
	fmt.Println("Please use `exit` or `Ctrl-D` to exit this program.")
	p := prompt.New(
		Executor,
		c.Complete,
		prompt.OptionTitle("KubeVela-prompt: interactive KubeVela client"),
		prompt.OptionPrefix(">>> "),
		prompt.OptionInputTextColor(prompt.Yellow),
		prompt.OptionCompletionWordSeparator(completer.FilePathCompletionSeparator),
	)
	p.Run()
}

// Executor executor velaql
func Executor(s string) {
	if s == "" {
		return
	} else if s == "exit" {
		fmt.Println("Bye!")
		os.Exit(0)
		return
	}

	array := strings.Split(s, "\\s+")
	if len(array) <= 2 {
		return
	}
	params := ""
	if array[1] != "" {
		params += array[1]
	}
	params += "{"
	for i := 2; i < len(array); i += 2 {
		cur := array[i] + "=" + array[i+1]
		params += cur
	}
	params += "}"

	if s == "" {
		return
	} else if s == "exit" {
		fmt.Println("Bye!")
		os.Exit(0)
		return
	}

	cmd := exec.Command("vela " + params)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		fmt.Printf("Got error: %s\n", err.Error())
	}
}

func excludeOptions(args []string) ([]string, bool) {
	l := len(args)
	if l == 0 {
		return nil, false
	}
	cmd := args[0]
	filtered := make([]string, 0, l)

	var skipNextArg bool
	for i := 0; i < len(args); i++ {
		if skipNextArg {
			skipNextArg = false
			continue
		}

		if cmd == "logs" && args[i] == "-f" {
			continue
		}

		for _, s := range []string{
			"-f", "--filename",
			"-n", "--namespace",
			"-s", "--server",
			"--kubeconfig",
			"--cluster",
			"--user",
			"-o", "--output",
			"-c",
			"--container",
		} {
			if strings.HasPrefix(args[i], s) {
				if strings.Contains(args[i], "=") {
					// we can specify option value like '-o=json'
					skipNextArg = false
				} else {
					skipNextArg = true
				}
				continue
			}
		}
		if strings.HasPrefix(args[i], "-") {
			continue
		}

		filtered = append(filtered, args[i])
	}
	return filtered, skipNextArg
}

var commands = []prompt.Suggest{
	{Text: "ql", Description: "Display one or many resources"},
	{Text: "exit", Description: "Exit this program"},
}

func getViews() ([]string, error) {
	var cm corev1.ConfigMapList
	ctx := context.Background()
	namespace := "vela-system"
	listOpts := []client.ListOption{
		client.InNamespace(namespace),
		client.MatchingLabels{
			oam.LabelConfigMapNamespace: namespace,
		},
	}
	if err := k8sClient.List(ctx, &cm, listOpts...); err != nil {
		return nil, errors.Wrapf(err, "failed to get configmaps")
	}

	var viewNames []string
	for _, t := range cm.Items {
		if t.Name != "" {
			// configmap name
			viewNames = append(viewNames, t.Name)
		}
	}
	return viewNames, nil

}

// getParamsFromView get parameters form view
func getParamsFromView(viewName string, namespace string) ([]string, error) {
	var cm corev1.ConfigMap
	var err error
	ctx := context.Background()
	key := client.ObjectKey{Namespace: namespace, Name: viewName}
	if err = k8sClient.Get(ctx, key, &cm); err != nil {
		return nil, errors.Wrapf(err, "failed to get configmaps %s", viewName)
	}
	var parameters []types.Parameter
	if temp, ok := cm.Data["template"]; temp != "" && ok {
		template := strings.Trim(temp, "｜")
		if parameters, err = cue.GetParameters(template, nil); err != nil {
			return nil, errors.Wrapf(err, "failed to get parameters %s", viewName)
		}
	}
	var requiredParams []string
	for _, val := range parameters {
		if val.Required {
			requiredParams = append(requiredParams, val.Name)
		}
	}
	return requiredParams, nil
}
