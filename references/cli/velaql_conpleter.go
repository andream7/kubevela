package cli

import (
	"context"
	"strings"

	errors2 "github.com/pkg/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/c-bata/go-prompt"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/kubernetes"
)

// NewCompleter new completer for velaql
func NewCompleter(ctx context.Context) (*Completer, error) {
	loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
	loader := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
		loadingRules,
		&clientcmd.ConfigOverrides{},
	)

	config, err := loader.ClientConfig()
	if err != nil {
		return nil, err
	}

	namespace, _, err := loader.Namespace()
	if err != nil {
		return nil, err
	}

	client, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	namespaces, err := client.CoreV1().Namespaces().List(ctx, metav1.ListOptions{})
	if err != nil {
		var e *errors.StatusError
		if ok := errors2.As(err, &e); ok && e.Status().Code == 403 {
			namespaces = nil
		} else {
			return nil, err
		}
	}

	return &Completer{
		namespace:     namespace,
		namespaceList: namespaces,
		client:        client,
	}, nil
}

// Completer Complete request
type Completer struct {
	namespace     string
	namespaceList *corev1.NamespaceList
	client        *kubernetes.Clientset
}

// Complete complete query
func (c *Completer) Complete(d prompt.Document) []prompt.Suggest {
	if d.TextBeforeCursor() == "" {
		return []prompt.Suggest{}
	}
	args := strings.Split(d.TextBeforeCursor(), " ")
	// w := d.GetWordBeforeCursor()

	// If PIPE is in text before the cursor, returns empty suggestions.
	for i := range args {
		if args[i] == "|" {
			return []prompt.Suggest{}
		}
	}

	// Return suggestions for option
	// if suggests, found := c.completeOptionArguments(d); found {
	// return suggests
	//}

	commandArgs, skipNext := excludeOptions(args)
	if skipNext {
		// when type 'get pod -o ', we don't want to complete pods. we want to type 'json' or other.
		// So we need to skip argumentCompleter.
		return []prompt.Suggest{}
	}
	return c.argumentsCompleter(c.namespace, commandArgs)
}

var params []string

func (c *Completer) argumentsCompleter(namespace string, args []string) []prompt.Suggest {
	if len(args) <= 1 {
		return prompt.FilterHasPrefix(commands, args[0], true)
	}

	first := args[0]
	if first == "ql" {
		second := args[1]
		views, _ := getViews()
		var resourceTypes []prompt.Suggest
		for _, view := range views {
			resourceTypes = append(resourceTypes, prompt.Suggest{
				Text: view},
			)
		}
		if len(args) == 2 {
			return prompt.FilterHasPrefix(resourceTypes, second, true)
		}

		if len(args) >= 3 {
			if len(params) == 0 {
				params, _ = getParamsFromView(args[2], namespace)
			}
			cur := params[0]
			params = params[0:]
			var resourceTypes = []prompt.Suggest{
				{Text: cur},
			}
			return prompt.FilterHasPrefix(resourceTypes, args[2], true)
		}
	}
	return []prompt.Suggest{}
}
