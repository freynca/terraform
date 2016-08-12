package azurerm

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/Azure/azure-sdk-for-go/arm/network"
	"github.com/hashicorp/terraform/helper/hashcode"
	"github.com/hashicorp/terraform/helper/schema"
)

func resourceArmLoadBalancer() *schema.Resource {
	return &schema.Resource{
		Create: resourceArmLoadBalancerCreate,
		Read:   resourceArmLoadBalancerRead,
		Update: resourceArmLoadBalancerCreate,
		Delete: resourceArmLoadBalancerDelete,

		Schema: map[string]*schema.Schema{
			"name": {
				Type:     schema.TypeString,
				Required: true,
				ForceNew: true,
			},

			"type": {
				Type:         schema.TypeString,
				Required:     true,
				ForceNew:     true,
				ValidateFunc: validateArmLoadBalancerType,
			},

			"location": {
				Type:      schema.TypeString,
				Optional:  true,
				ForceNew:  true,
				StateFunc: azureRMNormalizeLocation,
			},

			"resource_group_name": {
				Type:     schema.TypeString,
				Optional: true,
				ForceNew: true,
			},

			"frontend_ip_configuration": &schema.Schema{
				Type:     schema.TypeSet,
				Required: true,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"name": &schema.Schema{
							Type:     schema.TypeString,
							Required: true,
						},

						"private_ip_allocation_method": &schema.Schema{
							Type:     schema.TypeString,
							Optional: true,
						},

						"private_ip_address": &schema.Schema{
							Type:     schema.TypeString,
							Optional: true,
						},

						"subnet": &schema.Schema{
							Type:     schema.TypeString,
							Required: true,
						},
					},
				},

				Set: resourceArmLoadBalancerFrontEndIpConfigurationHash,
			},

			"backend_address_pool": &schema.Schema{
				Type:     schema.TypeSet,
				Required: true,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"name": &schema.Schema{
							Type:     schema.TypeString,
							Required: true,
						},
					},
				},
				Set: resourceArmLoadBalancerBackendAddressPoolHash,
			},

			"load_balancing_rule": &schema.Schema{
				Type:     schema.TypeSet,
				Required: true,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"name": &schema.Schema{
							Type:     schema.TypeString,
							Required: true,
						},
						"frontend_ip_configuration": &schema.Schema{
							Type:     schema.TypeString,
							Required: true,
						},
						"backend_address_pool": &schema.Schema{
							Type:     schema.TypeString,
							Required: true,
						},
						"probe": &schema.Schema{
							Type:     schema.TypeString,
							Required: true,
						},
						"protocol": &schema.Schema{
							Type:     schema.TypeString,
							Required: true,
						},
						"frontend_port": &schema.Schema{
							Type:     schema.TypeInt,
							Required: true,
						},
						"backend_port": &schema.Schema{
							Type:     schema.TypeInt,
							Required: true,
						},
						"idle_timeout_in_minutes": &schema.Schema{
							Type:     schema.TypeInt,
							Required: true,
						},
					},
				},
				Set: resourceArmLoadBalancerLoadBalancingRuleHash,
			},

			"probe": &schema.Schema{
				Type:     schema.TypeSet,
				Required: true,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"name": &schema.Schema{
							Type:     schema.TypeString,
							Required: true,
						},
						"protocol": &schema.Schema{
							Type:     schema.TypeString,
							Required: true,
						},
						"port": &schema.Schema{
							Type:     schema.TypeInt,
							Required: true,
						},
						"number_of_probes": &schema.Schema{
							Type:     schema.TypeInt,
							Required: true,
						},
						"interval_in_seconds": &schema.Schema{
							Type:     schema.TypeInt,
							Required: true,
						},
					},
				},
				Set: resourceArmLoadBalancerProbeHash,
			},

			"tags": tagsSchema(),
		},
	}
}

func validateArmLoadBalancerType(v interface{}, k string) (ws []string, es []error) {
	value := v.(string)

	if !strings.EqualFold(value, "internal") && !strings.EqualFold(value, "public") {
		es = append(es, fmt.Errorf("%q must be either Internal or Public", k))
	}

	return
}

func resourceArmLoadBalancerCreate(d *schema.ResourceData, meta interface{}) error {
	lbClient := meta.(*ArmClient).loadBalancerClient

	name := d.Get("name").(string)
	lbType := d.Get("type").(string)
	location := d.Get("location").(string)
	resGroup := d.Get("resource_group_name").(string)
	tags := d.Get("tags").(map[string]interface{})

	properties := network.LoadBalancerPropertiesFormat{}

	if _, ok := d.GetOk("frontend_ip_configuration"); ok {
		frontendConfigs, frontendConfigsErr := expandAzureRmLoadBalancerFrontendIPConfiguration(d)
		if frontendConfigsErr != nil {
			return fmt.Errorf("Error Building list of Frontend IP Configurations: %s", frontendConfigsErr)
		}
		if len(frontendConfigs) > 0 {
			properties.FrontendIPConfigurations = &frontendConfigs
		}
	}

	if _, ok := d.GetOk("backend_address_pool"); ok {
		backendAddressPools, backendAddressPoolsErr := expandAzureRmLoadBalancerBackendAddressPoolsConfiguration(d)
		if backendAddressPoolsErr != nil {
			return fmt.Errorf("Error Building list of Backend Address Pools: %s", backendAddressPoolsErr)
		}
		if len(backendAddressPools) > 0 {
			properties.BackendAddressPools = &backendAddressPools
		}
	}

	if _, ok := d.GetOk("load_balancing_rule"); ok {
		loadBalancingRules, loadBalancingRulesErr := expandAzureRmLoadBalancingRule(d)
		if loadBalancingRulesErr != nil {
			return fmt.Errorf("Error Building list of Load Balancing Rules: %s", loadBalancingRulesErr)
		}
		if len(loadBalancingRules) > 0 {
			properties.LoadBalancingRules = &loadBalancingRules
		}
	}

	if _, ok := d.GetOk("probe"); ok {
		loadBalancingProbes, loadBalancingProbesErr := expandAzureRmLoadBalancingProbe(d)
		if loadBalancingProbesErr != nil {
			return fmt.Errorf("Error Building list of Load Balancing Probe's: %s", loadBalancingProbesErr)
		}
		if len(loadBalancingProbes) > 0 {
			properties.Probes = &loadBalancingProbes
		}
	}

	loadBalancer := network.LoadBalancer{
		Name:       &name,
		Type:       &lbType,
		Location:   &location,
		Properties: &properties,
		Tags:       expandTags(tags),
	}

	_, err := lbClient.CreateOrUpdate(resGroup, name, loadBalancer, make(chan struct{}))
	if err != nil {
		return fmt.Errorf("Error creating Azure ARM Load Balancer '%s': %s", name, err)
	}

	read, err := lbClient.Get(resGroup, name, "")
	if err != nil {
		return err
	}
	if read.ID == nil {
		return fmt.Errorf("Cannot read Azure ARM Load Balancer %s (resource group %s) ID", name, resGroup)
	}

	d.SetId(*read.ID)

	return resourceArmLoadBalancerRead(d, meta)
}

// resourceArmLoadBalancerRead goes ahead and reads the state of the corresponding ARM load balancer.
func resourceArmLoadBalancerRead(d *schema.ResourceData, meta interface{}) error {
	return nil
}

// resourceArmLoadBalancerDelete deletes the specified ARM load balancer.
func resourceArmLoadBalancerDelete(d *schema.ResourceData, meta interface{}) error {
	return nil
}

// Helpers
func resourceArmLoadBalancerBackendAddressPoolHash(v interface{}) int {
	var buf bytes.Buffer
	m := v.(map[string]interface{})
	buf.WriteString(fmt.Sprintf("%s-", m["name"].(string)))

	return hashcode.String(buf.String())
}

func resourceArmLoadBalancerFrontEndIpConfigurationHash(v interface{}) int {
	var buf bytes.Buffer
	m := v.(map[string]interface{})
	buf.WriteString(fmt.Sprintf("%s-", m["name"].(string)))
	buf.WriteString(fmt.Sprintf("%s-", m["private_ip_allocation_method"].(string)))
	buf.WriteString(fmt.Sprintf("%s-", m["private_ip_address"].(string)))
	buf.WriteString(fmt.Sprintf("%s-", m["subnet"].(string)))

	return hashcode.String(buf.String())
}

func resourceArmLoadBalancerLoadBalancingRuleHash(v interface{}) int {
	var buf bytes.Buffer
	m := v.(map[string]interface{})
	buf.WriteString(fmt.Sprintf("%s-", m["name"].(string)))
	buf.WriteString(fmt.Sprintf("%s-", m["frontend_ip_configuration"].(string)))
	buf.WriteString(fmt.Sprintf("%s-", m["backend_address_pool"].(string)))
	buf.WriteString(fmt.Sprintf("%s-", m["probe"].(string)))
	buf.WriteString(fmt.Sprintf("%s-", m["protocol"].(string)))
	buf.WriteString(fmt.Sprintf("%d-", m["frontend_port"].(int)))
	buf.WriteString(fmt.Sprintf("%d-", m["backend_port"].(int)))
	buf.WriteString(fmt.Sprintf("%d-", m["idle_timeout_in_minutes"].(int)))

	return hashcode.String(buf.String())
}

func resourceArmLoadBalancerProbeHash(v interface{}) int {
	var buf bytes.Buffer
	m := v.(map[string]interface{})
	buf.WriteString(fmt.Sprintf("%s-", m["name"].(string)))
	buf.WriteString(fmt.Sprintf("%s-", m["protocol"].(string)))
	buf.WriteString(fmt.Sprintf("%d-", m["port"].(int)))
	buf.WriteString(fmt.Sprintf("%d-", m["number_of_probes"].(int)))
	buf.WriteString(fmt.Sprintf("%d-", m["interval_in_seconds"].(int)))

	return hashcode.String(buf.String())
}

// Parsers
func expandAzureRmLoadBalancerFrontendIPConfiguration(d *schema.ResourceData) ([]network.FrontendIPConfiguration, error) {

	configs := d.Get("frontend_ip_configuration").(*schema.Set).List()
	configurations := make([]network.FrontendIPConfiguration, 0, len(configs))

	for _, configRaw := range configs {
		data := configRaw.(map[string]interface{})

		private_ip_allocation_method := data["private_ip_allocation_method"].(string)
		private_ip_address := data["private_ip_address"].(string)
		subnet := data["subnet"].(string)

		properties := network.FrontendIPConfigurationPropertiesFormat{
			PrivateIPAddress:          &private_ip_address,
			PrivateIPAllocationMethod: network.IPAllocationMethod(private_ip_allocation_method),
			Subnet: &network.Subnet{
				ID: &subnet,
			},
			// TODO: Public LB's
			// PublicIPAddress: &public_ip_address
		}

		name := data["name"].(string)
		configuration := network.FrontendIPConfiguration{
			Name:       &name,
			Properties: &properties,
		}

		configurations = append(configurations, configuration)
	}

	return configurations, nil
}

func expandAzureRmLoadBalancerBackendAddressPoolsConfiguration(d *schema.ResourceData) ([]network.BackendAddressPool, error) {

	configs := d.Get("backend_address_pool").(*schema.Set).List()
	pools := make([]network.BackendAddressPool, 0, len(configs))

	for _, configRaw := range configs {
		data := configRaw.(map[string]interface{})

		name := data["name"].(string)
		pool := network.BackendAddressPool{
			Name: &name,
		}

		pools = append(pools, pool)
	}

	return pools, nil
}

func expandAzureRmLoadBalancingRule(d *schema.ResourceData) ([]network.LoadBalancingRule, error) {
	configs := d.Get("load_balancing_rule").(*schema.Set).List()
	rules := make([]network.LoadBalancingRule, 0, len(configs))

	for _, configRaw := range configs {
		data := configRaw.(map[string]interface{})

		protocol := data["protocol"].(string)
		loadDistribution := data["load_distribution"].(string)
		frontendPort := int32(data["frontend_port"].(int))
		backendPort := int32(data["backend_port"].(int))

		properties := network.LoadBalancingRulePropertiesFormat{
			Protocol:         network.TransportProtocol(protocol),
			LoadDistribution: network.LoadDistribution(loadDistribution),
			FrontendPort:     &frontendPort,
			BackendPort:      &backendPort,
		}

		if v, ok := d.GetOk("idle_timeout_in_minutes"); ok {
			idleTimeout := int32(v.(int))
			properties.IdleTimeoutInMinutes = &idleTimeout
		}

		if v, ok := d.GetOk("enable_floating_ip"); ok {
			enableFloatingIP := v.(bool)
			properties.EnableFloatingIP = &enableFloatingIP
		}

		name := data["name"].(string)
		rule := network.LoadBalancingRule{
			Name:       &name,
			Properties: &properties,
		}

		rules = append(rules, rule)
	}

	return rules, nil
}

func expandAzureRmLoadBalancingProbe(d *schema.ResourceData) ([]network.Probe, error) {
	configs := d.Get("probe").(*schema.Set).List()
	probes := make([]network.Probe, 0, len(configs))

	for _, configRaw := range configs {
		data := configRaw.(map[string]interface{})

		port := int32(d.Get("port").(int))
		interval := int32(d.Get("interval_in_seconds").(int))
		numberOfProbes := int32(d.Get("number_of_probes").(int))

		properties := network.ProbePropertiesFormat{
			Protocol:          network.ProbeProtocol(data["protocol"].(string)),
			Port:              &port,
			IntervalInSeconds: &interval,
			NumberOfProbes:    &numberOfProbes,
		}

		name := data["name"].(string)
		probe := network.Probe{
			Name:       &name,
			Properties: &properties,
		}

		probes = append(probes, probe)
	}

	return probes, nil
}
