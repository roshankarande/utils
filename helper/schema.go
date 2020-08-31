package helper

import (
	"github.com/hashicorp/terraform-plugin-sdk/helper/schema"
)

func SchemaCommandSpec() map[string]*schema.Schema {
	s := map[string]*schema.Schema{
		"command_before_create": {
			Type:        schema.TypeString,
			Optional:    true,
			Description: "Command to be triggered before creation of resource",
			Default:     "",
		},
		"command_after_create": {
			Type:        schema.TypeString,
			Optional:    true,
			Description: "Command to be triggered after creation of resource",
			Default:     "",
		},
		"command_before_delete": {
			Type:        schema.TypeString,
			Optional:    true,
			Description: "Command to be triggered before deleting the resource",
			Default:     "",
		},
		"command_after_delete": {
			Type:        schema.TypeString,
			Optional:    true,
			Description: "Command to be triggered after deleting the resource",
			Default:     "",
		},
		"command_before_read": {
			Type:        schema.TypeString,
			Optional:    true,
			Description: "Command to be triggered before reading of resource",
			Default:     "",
		},
		"command_after_read": {
			Type:        schema.TypeString,
			Optional:    true,
			Description: "Command to be triggered after reading of resource",
			Default:     "",
		},
		"command_before_update": {
			Type:        schema.TypeString,
			Optional:    true,
			Description: "Command to be triggered before updating of resource",
			Default:     "",
		},
		"command_after_update": {
			Type:        schema.TypeString,
			Optional:    true,
			Description: "Command to be triggered after updating of resource",
			Default:     "",
		},
	}

	return s
}

