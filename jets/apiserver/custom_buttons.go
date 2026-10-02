package main

import (
	"bytes"
	"encoding/json"
	"log"
	"os"
	"sync"
)

// customButtonsVariable names the deployment's own Pipeline Status buttons.
//
// jetstore_maintenance_02 Phase 1, defect D04, task AE.7 (2026-10-01). The CDK
// puts it in the UI service's environment (cdk/jetstore_one/stack/build_ui_service.go,
// uiEnvironment), this serves it at sign-in, and the React app appends the buttons
// to the table's last action row. It is a deployment setting so that no workspace
// document can name a deployment's buttons -- ui_refresh I-102's objection,
// which is why the table schemas refuse fromConfigRowActions.
const customButtonsVariable = "JETS_CUSTOM_BUTTONS_CONFIG_JSON"

// customButtons is the parsed value, read once per process.
//
// A variable, so a test can substitute its own; the apiserver never reassigns it.
var customButtons = sync.OnceValue(func() []json.RawMessage {
	return parseCustomButtons(os.Getenv(customButtonsVariable))
})

// parseCustomButtons turns the variable into the list the login response carries.
//
// **The whole value or nothing, and never a failed sign-in.** A value that is not
// a JSON array of objects is logged and served as an empty list, which is what
// the Flutter app did with the same variable (jetsclient/lib/button_config.dart,
// parseButtonConfig, returns [] on any FormatException): the buttons decorate one
// table, and a typo in a deployment variable must not lock every user out.
//
// Unset and empty are the same and are not logged. The CDK sets the entry from
// os.Getenv with no test, so a stack that never configured buttons still carries
// the variable, present and empty -- the case build_ui_service.go's comment on
// JETS_NO_GIT_ACCESS describes, and the reason this tests the value rather than
// its presence.
//
// The entries are passed through as the operator wrote them: what a button may
// say (type, key, label, file_path, fsk_params, replace_text/replace_with) is
// checked by the client that renders it, which drops an entry it cannot use. A
// second validator here would be a second definition of the shape to keep in step.
func parseCustomButtons(raw string) []json.RawMessage {
	none := []json.RawMessage{}
	if len(bytes.TrimSpace([]byte(raw))) == 0 {
		return none
	}
	var entries []json.RawMessage
	if err := json.Unmarshal([]byte(raw), &entries); err != nil {
		log.Printf("ENV %s is not a JSON array; serving no custom buttons: %v", customButtonsVariable, err)
		return none
	}
	for i, entry := range entries {
		var object map[string]json.RawMessage
		if err := json.Unmarshal(entry, &object); err != nil {
			log.Printf("ENV %s entry %d is not a JSON object; serving no custom buttons: %v",
				customButtonsVariable, i, err)
			return none
		}
	}
	return entries
}

// logCustomButtons says at startup how many buttons the variable configures, so
// an operator reads it in the log rather than by their absence on the screen.
func logCustomButtons() {
	log.Printf("ENV %s: %d custom button(s)", customButtonsVariable, len(customButtons()))
}
