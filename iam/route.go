package iam

import (
	"net/http"

	"github.com/sacloud/sakumock/core"
)

func (s *Server) routeTable() []core.RegisteredRoute {
	rl := func(h http.HandlerFunc) http.HandlerFunc {
		return s.rateLimiter.Middleware(core.GlobalKey(), h)
	}
	return []core.RegisteredRoute{
		// Users
		{Route: core.Route{Method: "GET", Path: "/compat/users", Description: "List users", Kind: "api"}, Handler: rl(s.handleListUsers)},
		{Route: core.Route{Method: "POST", Path: "/compat/users", Description: "Create a user", Kind: "api"}, Handler: rl(s.handleCreateUser)},
		{Route: core.Route{Method: "GET", Path: "/compat/users/{user_id}", Description: "Get a user", Kind: "api"}, Handler: rl(s.handleReadUser)},
		{Route: core.Route{Method: "PUT", Path: "/compat/users/{user_id}", Description: "Update a user", Kind: "api"}, Handler: rl(s.handleUpdateUser)},
		{Route: core.Route{Method: "DELETE", Path: "/compat/users/{user_id}", Description: "Delete a user", Kind: "api"}, Handler: rl(s.handleDeleteUser)},
		{Route: core.Route{Method: "POST", Path: "/compat/users/{user_id}/register-email", Description: "Register user email", Kind: "api"}, Handler: rl(s.handleRegisterEmail)},
		{Route: core.Route{Method: "POST", Path: "/compat/users/{user_id}/unregister-email", Description: "Unregister user email", Kind: "api"}, Handler: rl(s.handleUnregisterEmail)},
		{Route: core.Route{Method: "POST", Path: "/compat/users/{user_id}/deactivate-otp", Description: "Deactivate user OTP", Kind: "api"}, Handler: rl(s.handleDeactivateOTP)},
		{Route: core.Route{Method: "GET", Path: "/compat/users/{user_id}/trusted-devices", Description: "List trusted devices", Kind: "api"}, Handler: rl(s.handleListUserTrustedDevices)},
		{Route: core.Route{Method: "DELETE", Path: "/compat/users/{user_id}/trusted-devices/{trusted_device_id}", Description: "Delete trusted device", Kind: "api"}, Handler: rl(s.handleDeleteTrustedDevice)},
		{Route: core.Route{Method: "POST", Path: "/compat/users/{user_id}/clear-trusted-devices", Description: "Clear trusted devices", Kind: "api"}, Handler: rl(s.handleClearTrustedDevices)},
		{Route: core.Route{Method: "GET", Path: "/compat/users/{user_id}/security-keys", Description: "List security keys", Kind: "api"}, Handler: rl(s.handleListUserSecurityKeys)},
		{Route: core.Route{Method: "PUT", Path: "/compat/users/{user_id}/security-keys/{security_key_id}", Description: "Update security key", Kind: "api"}, Handler: rl(s.handleUpdateSecurityKey)},
		{Route: core.Route{Method: "DELETE", Path: "/compat/users/{user_id}/security-keys/{security_key_id}", Description: "Delete security key", Kind: "api"}, Handler: rl(s.handleDeleteSecurityKey)},

		// Groups
		{Route: core.Route{Method: "GET", Path: "/groups", Description: "List groups", Kind: "api"}, Handler: rl(s.handleListGroups)},
		{Route: core.Route{Method: "POST", Path: "/groups", Description: "Create a group", Kind: "api"}, Handler: rl(s.handleCreateGroup)},
		{Route: core.Route{Method: "GET", Path: "/groups/{group_id}", Description: "Get a group", Kind: "api"}, Handler: rl(s.handleReadGroup)},
		{Route: core.Route{Method: "PUT", Path: "/groups/{group_id}", Description: "Update a group", Kind: "api"}, Handler: rl(s.handleUpdateGroup)},
		{Route: core.Route{Method: "DELETE", Path: "/groups/{group_id}", Description: "Delete a group", Kind: "api"}, Handler: rl(s.handleDeleteGroup)},
		{Route: core.Route{Method: "GET", Path: "/groups/{group_id}/memberships", Description: "Get group memberships", Kind: "api"}, Handler: rl(s.handleReadMemberships)},
		{Route: core.Route{Method: "PUT", Path: "/groups/{group_id}/memberships", Description: "Update group memberships", Kind: "api"}, Handler: rl(s.handleUpdateMemberships)},

		// Projects
		{Route: core.Route{Method: "GET", Path: "/projects", Description: "List projects", Kind: "api"}, Handler: rl(s.handleListProjects)},
		{Route: core.Route{Method: "POST", Path: "/projects", Description: "Create a project", Kind: "api"}, Handler: rl(s.handleCreateProject)},
		{Route: core.Route{Method: "GET", Path: "/projects/{project_id}", Description: "Get a project", Kind: "api"}, Handler: rl(s.handleReadProject)},
		{Route: core.Route{Method: "PUT", Path: "/projects/{project_id}", Description: "Update a project", Kind: "api"}, Handler: rl(s.handleUpdateProject)},
		{Route: core.Route{Method: "DELETE", Path: "/projects/{project_id}", Description: "Delete a project", Kind: "api"}, Handler: rl(s.handleDeleteProject)},
		{Route: core.Route{Method: "POST", Path: "/move-projects", Description: "Move projects", Kind: "api"}, Handler: rl(s.handleMoveProjects)},
		{Route: core.Route{Method: "GET", Path: "/projects/{project_id}/iam-policy", Description: "Get project IAM policy", Kind: "api"}, Handler: rl(s.handleReadProjectIAMPolicy)},
		{Route: core.Route{Method: "PUT", Path: "/projects/{project_id}/iam-policy", Description: "Update project IAM policy", Kind: "api"}, Handler: rl(s.handleUpdateProjectIAMPolicy)},

		// Folders
		{Route: core.Route{Method: "GET", Path: "/folders", Description: "List folders", Kind: "api"}, Handler: rl(s.handleListFolders)},
		{Route: core.Route{Method: "POST", Path: "/folders", Description: "Create a folder", Kind: "api"}, Handler: rl(s.handleCreateFolder)},
		{Route: core.Route{Method: "GET", Path: "/folders/{folder_id}", Description: "Get a folder", Kind: "api"}, Handler: rl(s.handleReadFolder)},
		{Route: core.Route{Method: "PUT", Path: "/folders/{folder_id}", Description: "Update a folder", Kind: "api"}, Handler: rl(s.handleUpdateFolder)},
		{Route: core.Route{Method: "DELETE", Path: "/folders/{folder_id}", Description: "Delete a folder", Kind: "api"}, Handler: rl(s.handleDeleteFolder)},
		{Route: core.Route{Method: "POST", Path: "/move-folders", Description: "Move folders", Kind: "api"}, Handler: rl(s.handleMoveFolders)},
		{Route: core.Route{Method: "GET", Path: "/folders/{folder_id}/iam-policy", Description: "Get folder IAM policy", Kind: "api"}, Handler: rl(s.handleReadFolderIAMPolicy)},
		{Route: core.Route{Method: "PUT", Path: "/folders/{folder_id}/iam-policy", Description: "Update folder IAM policy", Kind: "api"}, Handler: rl(s.handleUpdateFolderIAMPolicy)},

		// Service Principals
		{Route: core.Route{Method: "GET", Path: "/service-principals", Description: "List service principals", Kind: "api"}, Handler: rl(s.handleListServicePrincipals)},
		{Route: core.Route{Method: "POST", Path: "/service-principals", Description: "Create a service principal", Kind: "api"}, Handler: rl(s.handleCreateServicePrincipal)},
		{Route: core.Route{Method: "GET", Path: "/service-principals/{service_principal_id}", Description: "Get a service principal", Kind: "api"}, Handler: rl(s.handleReadServicePrincipal)},
		{Route: core.Route{Method: "PUT", Path: "/service-principals/{service_principal_id}", Description: "Update a service principal", Kind: "api"}, Handler: rl(s.handleUpdateServicePrincipal)},
		{Route: core.Route{Method: "DELETE", Path: "/service-principals/{service_principal_id}", Description: "Delete a service principal", Kind: "api"}, Handler: rl(s.handleDeleteServicePrincipal)},
		{Route: core.Route{Method: "GET", Path: "/service-principals/{service_principal_id}/keys", Description: "List service principal keys", Kind: "api"}, Handler: rl(s.handleListSPKeys)},
		{Route: core.Route{Method: "POST", Path: "/service-principals/{service_principal_id}/upload-key", Description: "Upload public key", Kind: "api"}, Handler: rl(s.handleUploadSPKey)},
		{Route: core.Route{Method: "POST", Path: "/service-principals/{service_principal_id}/keys/{service_principal_key_id}/enable", Description: "Enable key", Kind: "api"}, Handler: rl(s.handleEnableSPKey)},
		{Route: core.Route{Method: "POST", Path: "/service-principals/{service_principal_id}/keys/{service_principal_key_id}/disable", Description: "Disable key", Kind: "api"}, Handler: rl(s.handleDisableSPKey)},
		{Route: core.Route{Method: "DELETE", Path: "/service-principals/{service_principal_id}/keys/{service_principal_key_id}", Description: "Delete key", Kind: "api"}, Handler: rl(s.handleDeleteSPKey)},
		{Route: core.Route{Method: "POST", Path: "/service-principals/oauth2/token", Description: "Issue OAuth2 token", Kind: "api"}, Handler: rl(s.handleOAuth2Token)},

		// Project API Keys
		{Route: core.Route{Method: "GET", Path: "/compat/api-keys", Description: "List API keys", Kind: "api"}, Handler: rl(s.handleListAPIKeys)},
		{Route: core.Route{Method: "POST", Path: "/compat/api-keys", Description: "Create an API key", Kind: "api"}, Handler: rl(s.handleCreateAPIKey)},
		{Route: core.Route{Method: "GET", Path: "/compat/api-keys/{apikey_id}", Description: "Get an API key", Kind: "api"}, Handler: rl(s.handleReadAPIKey)},
		{Route: core.Route{Method: "PUT", Path: "/compat/api-keys/{apikey_id}", Description: "Update an API key", Kind: "api"}, Handler: rl(s.handleUpdateAPIKey)},
		{Route: core.Route{Method: "DELETE", Path: "/compat/api-keys/{apikey_id}", Description: "Delete an API key", Kind: "api"}, Handler: rl(s.handleDeleteAPIKey)},

		// IAM Roles (read-only)
		{Route: core.Route{Method: "GET", Path: "/iam-roles", Description: "List IAM roles", Kind: "api"}, Handler: rl(s.handleListIAMRoles)},
		{Route: core.Route{Method: "GET", Path: "/iam-roles/{iam_role_id}", Description: "Get an IAM role", Kind: "api"}, Handler: rl(s.handleReadIAMRole)},

		// ID Roles (read-only)
		{Route: core.Route{Method: "GET", Path: "/id-roles", Description: "List ID roles", Kind: "api"}, Handler: rl(s.handleListIDRoles)},
		{Route: core.Route{Method: "GET", Path: "/id-roles/{id_role_id}", Description: "Get an ID role", Kind: "api"}, Handler: rl(s.handleReadIDRole)},

		// Organization IAM Policy
		{Route: core.Route{Method: "GET", Path: "/organization-iam-policy", Description: "Get organization IAM policy", Kind: "api"}, Handler: rl(s.handleReadOrgIAMPolicy)},
		{Route: core.Route{Method: "PUT", Path: "/organization-iam-policy", Description: "Update organization IAM policy", Kind: "api"}, Handler: rl(s.handleUpdateOrgIAMPolicy)},

		// Organization ID Policy
		{Route: core.Route{Method: "GET", Path: "/organization-id-policy", Description: "Get organization ID policy", Kind: "api"}, Handler: rl(s.handleReadOrgIDPolicy)},
		{Route: core.Route{Method: "PUT", Path: "/organization-id-policy", Description: "Update organization ID policy", Kind: "api"}, Handler: rl(s.handleUpdateOrgIDPolicy)},

		// Organization
		{Route: core.Route{Method: "GET", Path: "/organization", Description: "Get organization", Kind: "api"}, Handler: rl(s.handleReadOrganization)},
		{Route: core.Route{Method: "PUT", Path: "/organization", Description: "Update organization", Kind: "api"}, Handler: rl(s.handleUpdateOrganization)},

		// Password Policy
		{Route: core.Route{Method: "GET", Path: "/organization-password-policy", Description: "Get password policy", Kind: "api"}, Handler: rl(s.handleReadPasswordPolicy)},
		{Route: core.Route{Method: "PUT", Path: "/organization-password-policy", Description: "Update password policy", Kind: "api"}, Handler: rl(s.handleUpdatePasswordPolicy)},

		// Auth Conditions
		{Route: core.Route{Method: "GET", Path: "/organization-auth-conditions", Description: "Get auth conditions", Kind: "api"}, Handler: rl(s.handleReadAuthConditions)},
		{Route: core.Route{Method: "PUT", Path: "/organization-auth-conditions", Description: "Update auth conditions", Kind: "api"}, Handler: rl(s.handleUpdateAuthConditions)},

		// Auth Context
		{Route: core.Route{Method: "GET", Path: "/auth/context", Description: "Get auth context", Kind: "api"}, Handler: rl(s.handleAuthContext)},

		// SSO Profiles
		{Route: core.Route{Method: "GET", Path: "/sso-profiles", Description: "List SSO profiles", Kind: "api"}, Handler: rl(s.handleListSSOProfiles)},
		{Route: core.Route{Method: "POST", Path: "/sso-profiles", Description: "Create an SSO profile", Kind: "api"}, Handler: rl(s.handleCreateSSOProfile)},
		{Route: core.Route{Method: "GET", Path: "/sso-profiles/{sso_profile_id}", Description: "Get an SSO profile", Kind: "api"}, Handler: rl(s.handleReadSSOProfile)},
		{Route: core.Route{Method: "PUT", Path: "/sso-profiles/{sso_profile_id}", Description: "Update an SSO profile", Kind: "api"}, Handler: rl(s.handleUpdateSSOProfile)},
		{Route: core.Route{Method: "DELETE", Path: "/sso-profiles/{sso_profile_id}", Description: "Delete an SSO profile", Kind: "api"}, Handler: rl(s.handleDeleteSSOProfile)},
		{Route: core.Route{Method: "POST", Path: "/sso-profiles/{sso_profile_id}/assign", Description: "Assign SSO profile", Kind: "api"}, Handler: rl(s.handleAssignSSOProfile)},
		{Route: core.Route{Method: "POST", Path: "/sso-profiles/{sso_profile_id}/unassign", Description: "Unassign SSO profile", Kind: "api"}, Handler: rl(s.handleUnassignSSOProfile)},

		// SCIM Configurations
		{Route: core.Route{Method: "GET", Path: "/scim-configurations", Description: "List SCIM configurations", Kind: "api"}, Handler: rl(s.handleListScimConfigs)},
		{Route: core.Route{Method: "POST", Path: "/scim-configurations", Description: "Create a SCIM configuration", Kind: "api"}, Handler: rl(s.handleCreateScimConfig)},
		{Route: core.Route{Method: "GET", Path: "/scim-configurations/{id}", Description: "Get a SCIM configuration", Kind: "api"}, Handler: rl(s.handleReadScimConfig)},
		{Route: core.Route{Method: "PUT", Path: "/scim-configurations/{id}", Description: "Update a SCIM configuration", Kind: "api"}, Handler: rl(s.handleUpdateScimConfig)},
		{Route: core.Route{Method: "DELETE", Path: "/scim-configurations/{id}", Description: "Delete a SCIM configuration", Kind: "api"}, Handler: rl(s.handleDeleteScimConfig)},
		{Route: core.Route{Method: "POST", Path: "/scim-configurations/{id}/regenerate-token", Description: "Regenerate SCIM token", Kind: "api"}, Handler: rl(s.handleRegenerateScimToken)},

		// Service Policy
		{Route: core.Route{Method: "POST", Path: "/enable-service-policy", Description: "Enable service policy", Kind: "api"}, Handler: rl(s.handleEnableServicePolicy)},
		{Route: core.Route{Method: "POST", Path: "/disable-service-policy", Description: "Disable service policy", Kind: "api"}, Handler: rl(s.handleDisableServicePolicy)},
		{Route: core.Route{Method: "GET", Path: "/service-policy-status", Description: "Get service policy status", Kind: "api"}, Handler: rl(s.handleServicePolicyStatus)},
		{Route: core.Route{Method: "GET", Path: "/organization-service-policy", Description: "Get organization service policy", Kind: "api"}, Handler: rl(s.handleReadOrgServicePolicy)},
		{Route: core.Route{Method: "PUT", Path: "/organization-service-policy", Description: "Update organization service policy", Kind: "api"}, Handler: rl(s.handleUpdateOrgServicePolicy)},
		{Route: core.Route{Method: "GET", Path: "/service-policy-rule-templates", Description: "List service policy rule templates", Kind: "api"}, Handler: rl(s.handleServicePolicyRuleTemplates)},
	}
}
