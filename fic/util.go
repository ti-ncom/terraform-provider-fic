package fic

import (
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/nttcom/go-fic"

	"github.com/hashicorp/terraform-plugin-sdk/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/helper/schema"
	"github.com/unknwon/com"
)

// BuildRequest takes an opts struct and builds a request body for
// GO-FIC to execute
func BuildRequest(opts interface{}, parent string) (map[string]interface{}, error) {
	b, err := fic.BuildRequestBody(opts, "")
	if err != nil {
		return nil, err
	}

	b = AddValueSpecs(b)

	return map[string]interface{}{parent: b}, nil
}

// CheckDeleted checks the error to see if it's a 404 (Not Found) and, if so,
// sets the resource ID to the empty string instead of throwing an error.
func CheckDeleted(d *schema.ResourceData, err error, msg string) error {
	var e fic.ErrDefault404
	if errors.As(err, &e) {
		d.SetId("")
		return nil
	}

	return fmt.Errorf("%s: %w", msg, err)
}

// GetRegion returns the region that was specified in the resource. If a
// region was not set, the provider-level region is checked. The provider-level
// region can either be set by the region argument or by OS_REGION_NAME.
func GetRegion(d *schema.ResourceData, config *Config) string {
	if v, ok := d.GetOk("region"); ok {
		return v.(string)
	}

	return config.Region
}

// AddValueSpecs expands the 'value_specs' object and removes 'value_specs'
// from the request body.
func AddValueSpecs(body map[string]interface{}) map[string]interface{} {
	if body["value_specs"] != nil {
		for k, v := range body["value_specs"].(map[string]interface{}) {
			body[k] = v
		}
		delete(body, "value_specs")
	}

	return body
}

// MapValueSpecs converts ResourceData into a map
func MapValueSpecs(d *schema.ResourceData) map[string]string {
	m := make(map[string]string)
	for key, val := range d.Get("value_specs").(map[string]interface{}) {
		m[key] = val.(string)
	}
	return m
}

// List of headers that need to be redacted
var REDACT_HEADERS = []string{"x-auth-token", "x-auth-key", "x-service-token",
	"x-storage-token", "x-account-meta-temp-url-key", "x-account-meta-temp-url-key-2",
	"x-container-meta-temp-url-key", "x-container-meta-temp-url-key-2", "set-cookie",
	"x-subject-token"}

// RedactHeaders processes a headers object, returning a redacted list
func RedactHeaders(headers http.Header) (processedHeaders []string) {
	for name, header := range headers {
		for _, v := range header {
			if com.IsSliceContainsStr(REDACT_HEADERS, name) {
				processedHeaders = append(processedHeaders, fmt.Sprintf("%v: %v", name, "***"))
			} else {
				processedHeaders = append(processedHeaders, fmt.Sprintf("%v: %v", name, v))
			}
		}
	}
	return
}

// FormatHeaders processes a headers object plus a deliminator, returning a string
func FormatHeaders(headers http.Header, seperator string) string {
	redactedHeaders := RedactHeaders(headers)
	sort.Strings(redactedHeaders)

	return strings.Join(redactedHeaders, seperator)
}

func checkForRetryableError(err error) *resource.RetryError {
	switch errCode := err.(type) {
	case fic.ErrDefault500:
		return resource.RetryableError(err)
	case fic.ErrUnexpectedResponseCode:
		switch errCode.Actual {
		case 409, 503:
			return resource.RetryableError(err)
		default:
			return resource.NonRetryableError(err)
		}
	default:
		return resource.NonRetryableError(err)
	}
}

func suppressEquivilentTimeDiffs(k, old, new string, d *schema.ResourceData) bool {
	oldTime, err := time.Parse(time.RFC3339, old)
	if err != nil {
		return false
	}

	newTime, err := time.Parse(time.RFC3339, new)
	if err != nil {
		return false
	}

	return oldTime.Equal(newTime)
}

func resourceNetworkingAvailabilityZoneHintsV2(d *schema.ResourceData) []string {
	rawAZH := d.Get("availability_zone_hints").([]interface{})
	azh := make([]string, len(rawAZH))
	for i, raw := range rawAZH {
		azh[i] = raw.(string)
	}
	return azh
}

func expandVendorOptions(vendOptsRaw []interface{}) map[string]interface{} {
	vendorOptions := make(map[string]interface{})

	for _, option := range vendOptsRaw {
		for optKey, optValue := range option.(map[string]interface{}) {
			vendorOptions[optKey] = optValue
		}

	}

	return vendorOptions
}

func containerInfraLabelsMapV1(d *schema.ResourceData) (map[string]string, error) {
	m := make(map[string]string)
	for key, val := range d.Get("labels").(map[string]interface{}) {
		labelValue, ok := val.(string)
		if !ok {
			return nil, fmt.Errorf("label %s value should be string", key)
		}
		m[key] = labelValue
	}
	return m, nil
}

func containerInfraLabelsStringV1(d *schema.ResourceData) (string, error) {
	var formattedLabels string
	for key, val := range d.Get("labels").(map[string]interface{}) {
		labelValue, ok := val.(string)
		if !ok {
			return "", fmt.Errorf("label %s value should be string", key)
		}
		formattedLabels = strings.Join([]string{
			formattedLabels,
			fmt.Sprintf("%s=%s", key, labelValue),
		}, ",")
	}
	formattedLabels = strings.Trim(formattedLabels, ",")

	return formattedLabels, nil
}

func networkV2AttributesTags(d *schema.ResourceData) (tags []string) {
	rawTags := d.Get("tags").(*schema.Set).List()
	tags = make([]string, len(rawTags))

	for i, raw := range rawTags {
		tags[i] = raw.(string)
	}
	return
}

func repeatedString(baseString string, repeatCount int) string {
	return strings.Repeat(baseString, repeatCount)
}
