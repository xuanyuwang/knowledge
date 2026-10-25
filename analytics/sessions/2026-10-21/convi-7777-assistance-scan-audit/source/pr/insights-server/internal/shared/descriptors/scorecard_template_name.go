package descriptors

import (
	"fmt"

	"github.com/cresta/protoc-gen-resourcename/runtime"
)

/**
 * Cross revision scorecard template name
 */

const (
	ScorecardTemplateNameComponentIDCustomerID = "customer_id"
	ScorecardTemplateNameComponentIDProfileID  = "profile_id"
	ScorecardTemplateNameComponentIDTemplateID = "scorecard_template_id"
)

var ScorecardTemplateNameDesc = runtime.MustNewDescriptor(runtime.SinglePatternDescriptorParams{
	Type:    "cresta.v1.director.ScorecardTemplate",
	Pattern: "customers/{customer_id}/profiles/{profile_id}/scorecardTemplates/{scorecard_template_id}",
	Factory: func(components []string) (runtime.ResourceName, error) {
		return &ScorecardTemplateName{
			CustomerID: components[0],
			ProfileID:  components[1],
			TemplateID: components[2],
		}, nil
	},
})

// ScorecardTemplateNameInterface is the resource name interface.
type ScorecardTemplateNameInterface interface {
	runtime.ResourceName
	GetCustomerID() string
	GetProfileID() string
	GetTemplateID() string
}

func ScorecardTemplateNameToString(n ScorecardTemplateNameInterface) string {
	return fmt.Sprintf(
		ScorecardTemplateNameDesc.FmtPattern(),
		n.GetCustomerID(),
		n.GetProfileID(),
		n.GetTemplateID(),
	)
}

// ScorecardTemplateName is the resource name of a scorecard template.
type ScorecardTemplateName struct {
	CustomerID string
	ProfileID  string
	TemplateID string
}

// ParseScorecardTemplateName parses a string resource name of a scorecard template.
func ParseScorecardTemplateName(name string) (*ScorecardTemplateName, error) {
	resourceName, err := ScorecardTemplateNameDesc.Parse(name)
	if err != nil {
		return nil, err
	}
	return resourceName.(*ScorecardTemplateName), nil
}

// ParseScorecardTemplateNamePrefix parses a string resource name of a Conversation.
func ParseScorecardTemplateNamePrefix(name string) (*ScorecardTemplateName, error) {
	resourceName, err := ScorecardTemplateNameDesc.ParsePrefix(name)
	if err != nil {
		return nil, err
	}
	return resourceName.(*ScorecardTemplateName), nil
}

// CloneScorecardTemplateName creates a ScorecardTemplateName from ScorecardTemplateNameInterface.
func CloneScorecardTemplateName(src ScorecardTemplateNameInterface) *ScorecardTemplateName {
	return &ScorecardTemplateName{
		CustomerID: src.GetCustomerID(),
		ProfileID:  src.GetProfileID(),
		TemplateID: src.GetTemplateID(),
	}
}

func (r ScorecardTemplateName) String() string {
	return ScorecardTemplateNameToString(&r)
}

func (r *ScorecardTemplateName) GetCustomerID() string {
	return r.CustomerID
}

func (r *ScorecardTemplateName) GetProfileID() string {
	return r.ProfileID
}

func (r *ScorecardTemplateName) GetTemplateID() string {
	return r.TemplateID
}

func (r *ScorecardTemplateName) Descriptor() runtime.Descriptor {
	return ScorecardTemplateNameDesc
}

func (r *ScorecardTemplateName) ResourceString() string {
	return ScorecardTemplateNameToString(r)
}

func (r *ScorecardTemplateName) GetComponentByID(componentID string) string {
	switch componentID {
	case ScorecardTemplateNameComponentIDCustomerID:
		return r.GetCustomerID()
	case ScorecardTemplateNameComponentIDProfileID:
		return r.GetProfileID()
	case ScorecardTemplateNameComponentIDTemplateID:
		return r.GetTemplateID()
	default:
		return ""
	}
}

func (r *ScorecardTemplateName) GetComponentByIndex(componentIndex int) string {
	switch componentIndex {
	case 0:
		return r.GetCustomerID()
	case 1:
		return r.GetProfileID()
	case 2:
		return r.GetTemplateID()
	default:
		return ""
	}
}
