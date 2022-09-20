package cfsecurity

import (
	"code.cloudfoundry.org/cli/api/cloudcontroller/ccv3"
	"fmt"

	"github.com/hashicorp/go-uuid"
	"github.com/hashicorp/terraform-plugin-sdk/helper/schema"
	"github.com/orange-cloudfoundry/cf-security-entitlement/client"
)

func resourceBindAsg() *schema.Resource {

	return &schema.Resource{

		Create: resourceBindAsgCreate,
		Read:   resourceBindAsgRead,
		Update: resourceBindAsgUpdate,
		Delete: resourceBindAsgDelete,
		Importer: &schema.ResourceImporter{
			State: func(d *schema.ResourceData, meta interface{}) ([]*schema.ResourceData, error) {
				return []*schema.ResourceData{d}, nil
			},
		},

		Schema: map[string]*schema.Schema{
			"bind": &schema.Schema{
				Type:     schema.TypeSet,
				Optional: true,
				Set: func(v interface{}) int {
					elem := v.(map[string]interface{})
					str := fmt.Sprintf("%s-%s",
						elem["asg_id"],
						elem["space_id"],
					)
					return StringHashCode(str)
				},
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"asg_id": &schema.Schema{
							Type:     schema.TypeString,
							Required: true,
						},
						"space_id": &schema.Schema{
							Type:     schema.TypeString,
							Required: true,
						},
					},
				},
			},
			"force": &schema.Schema{
				Type:     schema.TypeBool,
				Optional: true,
			},
		},
	}
}

func resourceBindAsgCreate(d *schema.ResourceData, meta interface{}) error {
	manager := meta.(*Manager)

	err := refreshTokenIfExpired(manager)
	if err != nil {
		return err
	}

	id, err := uuid.GenerateUUID()
	if err != nil {
		return err
	}

	for _, elem := range getListOfStructs(d.Get("bind")) {
		err := manager.client.BindSecurityGroup(elem["asg_id"].(string), elem["space_id"].(string), manager.client.GetEndpoint())
		if err != nil {
			return err
		}
	}
	d.SetId(id)
	return nil
}

func resourceBindAsgRead(d *schema.ResourceData, meta interface{}) error {
	manager := meta.(*Manager)

	err := refreshTokenIfExpired(manager)
	if err != nil {
		return err
	}

	secGroups, err := manager.client.GetSecGroups([]ccv3.Query{}, 0)
	if err != nil {
		return err
	}

	userIsAdmin, _ := manager.client.CurrentUserIsAdmin()
	// check if force and if user is not an admin
	if d.Get("force").(bool) && !userIsAdmin {
		finalBinds := make([]map[string]interface{}, 0)
		for i, secGroup := range secGroups.Resources {
			_ = manager.client.AddSecGroupRelationShips(&secGroups.Resources[i])
			for _, space := range secGroups.Resources[i].Relationships.RunningSpaces.Data {
				finalBinds = append(finalBinds, map[string]interface{}{
					"asg_id":   secGroup.GUID,
					"space_id": space.GUID,
				})
			}
		}
		d.Set("bind", finalBinds)
		return nil
	}

	secGroupsTf := getListOfStructs(d.Get("bind"))
	finalBinds := intersectSlices(secGroupsTf, secGroups.Resources, func(source, item interface{}) bool {
		secGroupTf := source.(map[string]interface{})
		secGroup := item.(client.SecurityGroup)
		asgIDTf := secGroupTf["asg_id"].(string)
		spaceIDTf := secGroupTf["space_id"].(string)
		if asgIDTf != secGroup.GUID {
			return false
		}
		spaces, _ := manager.client.GetSecGroupSpaces(&secGroup)
		return isInSlice(spaces.Resources, func(object interface{}) bool {
			space := object.(client.Space)
			return space.GUID == spaceIDTf
		})
	})
	d.Set("bind", finalBinds)
	return nil
}

func resourceBindAsgUpdate(d *schema.ResourceData, meta interface{}) error {
	manager := meta.(*Manager)

	err := refreshTokenIfExpired(manager)
	if err != nil {
		return err
	}

	old, now := d.GetChange("bind")
	remove, add := getListMapChanges(old, now, func(source, item map[string]interface{}) bool {
		return source["asg_id"] == item["asg_id"] &&
			source["space_id"] == item["space_id"]
	})
	if len(remove) > 0 {
		for _, bind := range remove {
			err := manager.client.UnBindSecurityGroup(bind["asg_id"].(string), bind["space_id"].(string), manager.client.GetEndpoint())
			if err != nil && !isNotFoundErr(err) {
				return err
			}
		}

	}
	if len(add) > 0 {
		for _, bind := range add {
			err := manager.client.BindSecurityGroup(bind["asg_id"].(string), bind["space_id"].(string), manager.client.GetEndpoint())
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func resourceBindAsgDelete(d *schema.ResourceData, meta interface{}) error {
	manager := meta.(*Manager)

	err := refreshTokenIfExpired(manager)
	if err != nil {
		return err
	}

	for _, elem := range getListOfStructs(d.Get("bind")) {
		err := manager.client.UnBindSecurityGroup(elem["asg_id"].(string), elem["space_id"].(string), manager.client.GetEndpoint())
		if err != nil && !isNotFoundErr(err) {
			return err
		}
	}
	return nil
}
