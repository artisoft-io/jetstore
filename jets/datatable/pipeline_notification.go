package datatable

import (
	"log"
	"os"
	"strings"

	"github.com/artisoft-io/jetstore/jets/utils"
)

func (ca *StatusUpdate) notifyApiGateway(schemaProvider *SchemaProviderShort) error {
	// NOTE 2024-05-13 Added Notification to API Gateway via env var CPIPES_STATUS_NOTIFICATION_ENDPOINT
	// or CPIPES_STATUS_NOTIFICATION_ENDPOINT_JSON
	// ALSO set a deadline to calls to database to avoid locks, don't fail the call when database fails
	apiEndpoint := os.Getenv("CPIPES_STATUS_NOTIFICATION_ENDPOINT")
	apiEndpointJson := os.Getenv("CPIPES_STATUS_NOTIFICATION_ENDPOINT_JSON")
	if apiEndpoint == "" && apiEndpointJson == "" {
		return nil
	}
	override := ca.NotifyApiGatewayOverride
	switch {
		case override == "no_notifications" || override == "start_only" || (override == "failure_only" && ca.Status != "failed"):
			log.Printf("%s CPIPES_STATUS_NOTIFICATION: skipping completed/failed notification to API Gateway as notify_api_gateway_override is set to '%s'\n", 
				ca.SessionId, override)
			return nil
	}
	var notificationTemplate string
	var errMsg string
	customFileKeys := make([]string, 0)
	ck := os.Getenv("CPIPES_CUSTOM_FILE_KEY_NOTIFICATION")
	if len(ck) > 0 {
		customFileKeys = strings.Split(ck, ",")
	}
	if schemaProvider != nil {
		if schemaProvider.NotificationTemplatesOverrides != nil {
			if ca.Status == "failed" {
				notificationTemplate = schemaProvider.NotificationTemplatesOverrides["CPIPES_FAILED_NOTIFICATION_JSON"]
				errMsg = ca.FailureDetails
			} else {
				notificationTemplate = schemaProvider.NotificationTemplatesOverrides["CPIPES_COMPLETED_NOTIFICATION_JSON"]
			}
		}
		if len(schemaProvider.NotificationRoutingOverridesJson) > 0 {
			apiEndpointJson = schemaProvider.NotificationRoutingOverridesJson
		}
	}
	// Get the template defined at deployment if no override was found
	if len(notificationTemplate) == 0 {
		if ca.Status == "failed" {
			notificationTemplate = os.Getenv("CPIPES_FAILED_NOTIFICATION_JSON")
			errMsg = ca.FailureDetails
		} else {
			notificationTemplate = os.Getenv("CPIPES_COMPLETED_NOTIFICATION_JSON")
		}
	}
	return utils.DoNotifyApiGateway(ca.FileKey, apiEndpoint, apiEndpointJson, notificationTemplate, customFileKeys, errMsg, ca.CpipesEnv)
}
