package models

type RoleType string

// Don't change the names since these values are usually saved in the
// database
const (
	RoleTypeUndefined   RoleType = "undefined"
	RoleTypeNormal      RoleType = "normal"
	RoleTypeCircle      RoleType = "circle"
	RoleTypeLeadLink    RoleType = "leadlink"
	RoleTypeRepLink     RoleType = "replink"
	RoleTypeFacilitator RoleType = "facilitator"
	RoleTypeSecretary   RoleType = "secretary"
)

func (r RoleType) IsCoreRoleType() bool {
	return r == RoleTypeLeadLink ||
		r == RoleTypeRepLink ||
		r == RoleTypeFacilitator ||
		r == RoleTypeSecretary
}

func (r RoleType) String() string {
	return string(r)
}

func RoleTypeFromString(r string) RoleType {
	switch r {
	case "normal":
		return RoleTypeNormal
	case "circle":
		return RoleTypeCircle
	case "leadlink":
		return RoleTypeLeadLink
	case "replink":
		return RoleTypeRepLink
	case "facilitator":
		return RoleTypeFacilitator
	case "secretary":
		return RoleTypeSecretary
	default:
		return RoleTypeUndefined
	}
}

type Role struct {
	Vertex
	RoleType RoleType
	Depth    int32
	Name     string
	Purpose  string
}

type RoleAdditionalContent struct {
	Vertex
	Content string
}

type MemberCirclePermissions struct {
	AssignChildCircleLeadLink   bool
	AssignChildRoleMembers      bool
	ManageChildRoles            bool
	AssignCircleDirectMembers   bool
	AssignCircleCoreRoles       bool
	ManageRoleAdditionalContent bool
	// special cases for root circle
	AssignRootCircleLeadLink bool
	ManageRootCircle         bool
}

type CoreRoleDefinition struct {
	Role             *Role
	Domains          []*Domain
	Accountabilities []*Accountability
}

func GetCoreRoles() []*CoreRoleDefinition {
	return []*CoreRoleDefinition{
		{
			Role: &Role{
				Name:     "Lead Link",
				RoleType: RoleTypeLeadLink,
				Purpose:  "The Lead Link holds the Purpose of the overall Circle",
			},
			Domains: []*Domain{
				{Description: "Role assignments within the Circle"},
			},
			Accountabilities: []*Accountability{
				{Description: "Structuring the Governance of the Circle to enact its Purpose and Accountabilities"},
				{Description: "Assigning Partners to the Circle’s Roles; monitoring the fit; offering feedback to enhance fit; and re-assigning Roles to other Partners when useful for enhancing fit"},
				{Description: "Allocating the Circle’s resources across its various Projects and/or Roles"},
				{Description: "Establishing priorities and Strategies for the Circle"},
				{Description: "Defining metrics for the circle"},
				{Description: "Removing constraints within the Circle to the Super-Circle enacting its Purpose and Accountabilities"},
			},
		},
		{
			Role: &Role{
				Name:     "Rep Link",
				RoleType: RoleTypeRepLink,
				Purpose:  "Within the Super-Circle, the Rep Link holds the Purpose of the SubCircle; within the Sub-Circle, the Rep Link’s Purpose is: Tensions relevant to process in the Super-Circle channeled out and resolved",
			},
			Accountabilities: []*Accountability{
				{Description: "Removing constraints within the broader Organization that limit the Sub-Circle"},
				{Description: "Seeking to understand Tensions conveyed by Sub-Circle Circle Members, and discerning those appropriate to process in the Super-Circle"},
				{Description: "Providing visibility to the Super-Circle into the health of the Sub-Circle, including reporting on any metrics or checklist items assigned to the whole Sub-Circle"},
			},
		},
		{
			Role: &Role{
				Name:     "Facilitator",
				RoleType: RoleTypeFacilitator,
				Purpose:  "Circle governance and operational practices aligned with the Constitution",
			},
			Accountabilities: []*Accountability{
				{Description: "Facilitating the Circle’s constitutionally-required meetings"},
				{Description: "Auditing the meetings and records of Sub-Circles as needed, and declaring a Process Breakdown upon discovering a pattern of behavior that conflicts with the rules of the Constitution"},
			},
		},
		{
			Role: &Role{
				Name:     "Secretary",
				RoleType: RoleTypeSecretary,
				Purpose:  "Steward and stabilize the Circle’s formal records and record-keeping process",
			},
			Domains: []*Domain{
				{Description: "All constitutionally-required records of the Circle"},
			},
			Accountabilities: []*Accountability{
				{Description: "Scheduling the Circle’s required meetings, and notifying all Core Circle Members of scheduled times and locations"},
				{Description: "Capturing and publishing the outputs of the Circle’s required meetings, and maintaining a compiled view of the Circle’s current Governance, checklist items, and metrics"},
				{Description: "Interpreting Governance and the Constitution upon request"},
			},
		},
	}
}
