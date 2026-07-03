package utils

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"
)

// Perform notification to an api endpoint using a json template.
// The endpoint is specified by either:
//   - apiEndpoint: a single endpoint specified by env var CPIPES_STATUS_NOTIFICATION_ENDPOINT, or
//   - apiEndpointJson: a json with the routing info specified by env
//     var CPIPES_STATUS_NOTIFICATION_ENDPOINT_JSON or by the schema provider (NotificationRoutingOverridesJson) with the following format:
//
// Note apiEndpoint takes presendence over apiEndpointJson when both are specified.
// Also, the schema provider can override apiEndpointJson (does not override apiEndpoint)
// and the notification template defined at deployment via env var.
func DoNotifyApiGateway(fileKey, apiEndpoint, apiEndpointJson, notificationTemplate string,
	customFileKeys []string, errMsg string, envSettings map[string]any) error {
	var (
		ctx    context.Context
		cancel context.CancelFunc
	)
	if apiEndpoint == "" && apiEndpointJson == "" {
		log.Println("error: no endpoints defined for DoNotifyApiGateway")
		return fmt.Errorf("error: no endpoints defined for DoNotifyApiGateway")
	}
	timeout, err := time.ParseDuration("10s")
	if err == nil {
		// The request has a timeout, so create a context that is
		// canceled automatically when the timeout expires.
		ctx, cancel = context.WithTimeout(context.Background(), timeout)
	} else {
		ctx, cancel = context.WithCancel(context.Background())
	}
	defer cancel() // Cancel ctx as soon as DoNotifyApiGateway returns.
	// Prepare the API request.
	var value string
	// Extract file key components
	fileKeyComponents := make(map[string]any)
	fileKeyComponents = SplitFileKeyIntoComponents(fileKeyComponents, &fileKey)
	v := fileKeyComponents["client"]
	if v != nil {
		notificationTemplate = strings.ReplaceAll(notificationTemplate, "{{client}}", v.(string))
	} else {
		notificationTemplate = strings.ReplaceAll(notificationTemplate, "{{client}}", "")
	}
	v = fileKeyComponents["org"]
	if v != nil {
		notificationTemplate = strings.ReplaceAll(notificationTemplate, "{{org}}", v.(string))
	} else {
		notificationTemplate = strings.ReplaceAll(notificationTemplate, "{{org}}", "")
	}
	v = fileKeyComponents["object_type"]
	if v != nil {
		notificationTemplate = strings.ReplaceAll(notificationTemplate, "{{object_type}}", v.(string))
	} else {
		notificationTemplate = strings.ReplaceAll(notificationTemplate, "{{object_type}}", "")
	}
	for _, key := range customFileKeys {
		switch vv := fileKeyComponents[key].(type) {
		case string:
			value = vv
		default:
			value = ""
		}
		value = strings.ReplaceAll(value, `"`, `\"`)
		notificationTemplate = strings.ReplaceAll(notificationTemplate, fmt.Sprintf("{{%s}}", key), value)
		apiEndpointJson = strings.ReplaceAll(apiEndpointJson, fmt.Sprintf("{{%s}}", key), value)
	}

	if len(errMsg) > 0 {
		errMsg = strings.ReplaceAll(errMsg, `"`, `\"`)
		notificationTemplate = strings.ReplaceAll(notificationTemplate, "{{error}}", errMsg)
	}

	// Do substitution using key/value provided by cpipes config and main schema provider
replaceEnv:
	for key, value := range envSettings {
		str, ok := value.(string)
		if ok {
			var replace string
			switch {
			case strings.HasPrefix(key, "${"):
				replace = fmt.Sprintf("{%s}", key[1:])
			case strings.HasPrefix(key, "$"):
				replace = fmt.Sprintf("{{%s}}", key[1:])
			default:
				continue replaceEnv
			}
			notificationTemplate = strings.ReplaceAll(notificationTemplate, replace, str)
			if len(apiEndpoint) == 0 {
				apiEndpointJson = strings.ReplaceAll(apiEndpointJson, replace, str)
			}
		}
	}
	// remove residual unreplaced placeholder in the template to avoid confusion on the receiving end
	notificationTemplate = RemoveUnreplacedPlaceholder(notificationTemplate)
	apiEndpointJson = RemoveUnreplacedPlaceholder(apiEndpointJson)
	log.Println("Final notificationTemplate after substitution:", notificationTemplate)
	log.Println("Final apiEndpointJson after substitution:", apiEndpointJson)

	// Identify the endpoint where to send the request
	if len(apiEndpoint) == 0 {
		routes := make(map[string]string)
		err = json.Unmarshal([]byte(apiEndpointJson), &routes)
		if err != nil {
			err = fmt.Errorf("while parsing CPIPES_STATUS_NOTIFICATION_ENDPOINT_JSON: %v", err)
			log.Println(err)
			return err
		}
		// key := routes["key"]
		// altKey := routes["alt_key"]
		if len(routes["key"]) == 0 && len(routes["alt_key"]) == 0 {
			log.Println("Invalid routing json, key and alt_key are both missing, need at leat one to be set.")
			return fmt.Errorf("error: invalid routing json, key and alt_key are missing, need at least one to be set")
		}
		keys := []string{routes["key"], routes["alt_key"]}
		for _, key := range keys {
			if len(key) == 0 {
				continue
			}
			// Check if it's a fileKeyComponents
			routingObj := fileKeyComponents[key]
			routingKey, ok := routingObj.(string)
			if ok {
				apiEndpoint = routes[strings.ToUpper(routingKey)]
				if len(apiEndpoint) > 0 {
					break
				}
			}
			// Check if can route with key
			apiEndpoint = routes[strings.ToUpper(key)]
			if len(apiEndpoint) > 0 {
				break
			}
		}

		if len(apiEndpoint) == 0 {
			err = fmt.Errorf("error: notification endpoint not found for routing keys: %v", keys)
			log.Println(err)
			return err
		}
	}

	log.Println("POST Request:", notificationTemplate)
	log.Println("TO:", apiEndpoint)
	req, err := http.NewRequest("POST", apiEndpoint, bytes.NewBuffer([]byte(notificationTemplate)))
	if err != nil {
		log.Println(err)
		return err
	}
	req.Header.Add("Content-Type", "application/json")
	req = req.WithContext(ctx)
	client := &http.Client{}
	res, err := client.Do(req)
	if err != nil {
		err = fmt.Errorf("while posting result to api gateway: %v", err)
		log.Println(err)
		return err
	}
	log.Println("Result for posting status to api gateway:", res.StatusCode, res.Status)
	res.Body.Close()
	return nil
}
