package cfsecurity

import (
	"fmt"

	"github.com/hashicorp/go-uuid"
	"github.com/hashicorp/terraform-plugin-sdk/helper/schema"
	"github.com/orange-cloudfoundry/cf-security-entitlement/model"
)

func resourceEntitleAsg() *schema.Resource {

	return &schema.Resource{

		Create: resourceEntitleAsgCreate,
		Update: resourceEntitleAsgUpdate,
		Read:   resourceEntitleAsgRead,
		Delete: resourceEntitleAsgDelete,
		Importer: &schema.ResourceImporter{
			State: func(d *schema.ResourceData, meta interface{}) ([]*schema.ResourceData, error) {
				return []*schema.ResourceData{d}, nil
			},
		},

		Schema: map[string]*schema.Schema{
			"entitle": &schema.Schema{
				Type:     schema.TypeSet,
				Optional: true,
				Set: func(v interface{}) int {
					elem := v.(map[string]interface{})
					str := fmt.Sprintf("%s-%s",
						elem["asg_id"],
						elem["org_id"],
					)
					return StringHashCode(str)
				},
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"asg_id": &schema.Schema{
							Type:     schema.TypeString,
							Required: true,
						},
						"org_id": &schema.Schema{
							Type:     schema.TypeString,
							Required: true,
						},
					},
				},
			},
		},
	}
}

func resourceEntitleAsgCreate(d *schema.ResourceData, meta interface{}) error {
	manager := meta.(*Manager)

	err := refreshTokenIfExpired(manager)
	if err != nil {
		return err
	}

	id, err := uuid.GenerateUUID()
	if err != nil {
		return err
	}
	for _, elem := range getListOfStructs(d.Get("entitle")) {
		err := manager.client.EntitleSecurityGroup(elem["asg_id"].(string), elem["org_id"].(string))
		if err != nil {
			return err
		}
	}
	d.SetId(id)
	return nil
}

func resourceEntitleAsgRead(d *schema.ResourceData, meta interface{}) error {
	manager := meta.(*Manager)

	err := refreshTokenIfExpired(manager)
	if err != nil {
		return err
	}

	entitlements, err := manager.client.GetSecGroupEntitlements()
	if err != nil {
		return err
	}
	entitlementsTf := getListOfStructs(d.Get("entitle"))
	finalEntitlements := intersectSlices(entitlementsTf, entitlements, func(source, item interface{}) bool {
		entitlementTf := source.(map[string]interface{})
		entitlement := item.(model.EntitlementSecGroup)
		return entitlementTf["asg_id"].(string) == entitlement.SecurityGroupGUID &&
			entitlementTf["org_id"] == entitlement.OrganizationGUID
	})
	return d.Set("entitle", finalEntitlements)
}

func resourceEntitleAsgUpdate(d *schema.ResourceData, meta interface{}) error {
	manager := meta.(*Manager)

	err := refreshTokenIfExpired(manager)
	if err != nil {
		return err
	}

	old, now := d.GetChange("entitle")
	remove, add := getListMapChanges(old, now, func(source, item map[string]interface{}) bool {
		return source["asg_id"] == item["asg_id"] &&
			source["org_id"] == item["org_id"]
	})
	if len(remove) > 0 {
		for _, r := range remove {
			err := manager.client.RevokeSecurityGroup(r["asg_id"].(string), r["org_id"].(string))
			if err != nil && !isNotFoundErr(err) {
				return err
			}
		}

	}
	if len(add) > 0 {
		for _, a := range add {
			err := manager.client.EntitleSecurityGroup(a["asg_id"].(string), a["org_id"].(string))
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func resourceEntitleAsgDelete(d *schema.ResourceData, meta interface{}) error {
	manager := meta.(*Manager)
	err := refreshTokenIfExpired(manager)
	if err != nil {
		return err
	}
	for _, elem := range getListOfStructs(d.Get("entitle")) {
		err := manager.client.RevokeSecurityGroup(elem["asg_id"].(string), elem["org_id"].(string))
		if err != nil && !isNotFoundErr(err) {
			return err
		}
	}
	return nil
}
