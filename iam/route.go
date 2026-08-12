package iam

import (
	"net/http"

	"github.com/sacloud/sakumock/core"
)

func (s *Server) routeTable() []core.RegisteredRoute {
	rl := func(h http.HandlerFunc) http.HandlerFunc {
		return s.rateLimiter.Middleware(core.GlobalKey(), h)
	}
	route := func(method, path, desc string, h http.HandlerFunc) core.RegisteredRoute {
		return core.RegisteredRoute{
			Route: core.Route{Method: method, Path: path, Description: desc, Kind: "api"},
			// Fault injection outermost (an injected fault is an
			// infrastructure-level failure, so it may mask a would-be 429/400),
			// then rate limit, then spec-derived body validation, then the
			// handler. Response validation sits innermost so only what the
			// handler itself produces is checked against the spec.
			Handler: s.fault.Middleware(rl(s.validator.Middleware(method, path, s.respValidator.Middleware(method, path, h)))),
		}
	}
	table := []core.RegisteredRoute{
		// Users
		route("GET", "/compat/users", "List users", s.handleListUsers),
		route("POST", "/compat/users", "Create a user", s.handleCreateUser),
		route("GET", "/compat/users/{user_id}", "Get a user", s.handleReadUser),
		route("PUT", "/compat/users/{user_id}", "Update a user", s.handleUpdateUser),
		route("DELETE", "/compat/users/{user_id}", "Delete a user", s.handleDeleteUser),
		route("POST", "/compat/users/{user_id}/register-email", "Register user email", s.handleRegisterEmail),
		route("POST", "/compat/users/{user_id}/unregister-email", "Unregister user email", s.handleUnregisterEmail),
		route("POST", "/compat/users/{user_id}/deactivate-otp", "Deactivate user OTP", s.handleDeactivateOTP),
		route("GET", "/compat/users/{user_id}/trusted-devices", "List trusted devices", s.handleListUserTrustedDevices),
		route("DELETE", "/compat/users/{user_id}/trusted-devices/{trusted_device_id}", "Delete trusted device", s.handleDeleteTrustedDevice),
		route("POST", "/compat/users/{user_id}/clear-trusted-devices", "Clear trusted devices", s.handleClearTrustedDevices),
		route("GET", "/compat/users/{user_id}/security-keys", "List security keys", s.handleListUserSecurityKeys),
		route("GET", "/compat/users/{user_id}/security-keys/{security_key_id}", "Get a security key", s.handleReadSecurityKey),
		route("PUT", "/compat/users/{user_id}/security-keys/{security_key_id}", "Update security key", s.handleUpdateSecurityKey),
		route("DELETE", "/compat/users/{user_id}/security-keys/{security_key_id}", "Delete security key", s.handleDeleteSecurityKey),

		// Groups
		route("GET", "/groups", "List groups", s.handleListGroups),
		route("POST", "/groups", "Create a group", s.handleCreateGroup),
		route("GET", "/groups/{group_id}", "Get a group", s.handleReadGroup),
		route("PUT", "/groups/{group_id}", "Update a group", s.handleUpdateGroup),
		route("DELETE", "/groups/{group_id}", "Delete a group", s.handleDeleteGroup),
		route("GET", "/groups/{group_id}/memberships", "Get group memberships", s.handleReadMemberships),
		route("PUT", "/groups/{group_id}/memberships", "Update group memberships", s.handleUpdateMemberships),

		// Projects
		route("GET", "/projects", "List projects", s.handleListProjects),
		route("POST", "/projects", "Create a project", s.handleCreateProject),
		route("GET", "/projects/{project_id}", "Get a project", s.handleReadProject),
		route("PUT", "/projects/{project_id}", "Update a project", s.handleUpdateProject),
		route("DELETE", "/projects/{project_id}", "Delete a project", s.handleDeleteProject),
		route("POST", "/move-projects", "Move projects", s.handleMoveProjects),
		route("GET", "/projects/{project_id}/iam-policy", "Get project IAM policy", s.handleReadProjectIAMPolicy),
		route("PUT", "/projects/{project_id}/iam-policy", "Update project IAM policy", s.handleUpdateProjectIAMPolicy),

		// Folders
		route("GET", "/folders", "List folders", s.handleListFolders),
		route("POST", "/folders", "Create a folder", s.handleCreateFolder),
		route("GET", "/folders/{folder_id}", "Get a folder", s.handleReadFolder),
		route("PUT", "/folders/{folder_id}", "Update a folder", s.handleUpdateFolder),
		route("DELETE", "/folders/{folder_id}", "Delete a folder", s.handleDeleteFolder),
		route("POST", "/move-folders", "Move folders", s.handleMoveFolders),
		route("GET", "/folders/{folder_id}/iam-policy", "Get folder IAM policy", s.handleReadFolderIAMPolicy),
		route("PUT", "/folders/{folder_id}/iam-policy", "Update folder IAM policy", s.handleUpdateFolderIAMPolicy),

		// Service Principals
		route("GET", "/service-principals", "List service principals", s.handleListServicePrincipals),
		route("POST", "/service-principals", "Create a service principal", s.handleCreateServicePrincipal),
		route("GET", "/service-principals/{service_principal_id}", "Get a service principal", s.handleReadServicePrincipal),
		route("PUT", "/service-principals/{service_principal_id}", "Update a service principal", s.handleUpdateServicePrincipal),
		route("DELETE", "/service-principals/{service_principal_id}", "Delete a service principal", s.handleDeleteServicePrincipal),
		route("GET", "/service-principals/{service_principal_id}/keys", "List service principal keys", s.handleListSPKeys),
		route("POST", "/service-principals/{service_principal_id}/upload-key", "Upload public key", s.handleUploadSPKey),
		route("POST", "/service-principals/{service_principal_id}/keys/{service_principal_key_id}/enable", "Enable key", s.handleEnableSPKey),
		route("POST", "/service-principals/{service_principal_id}/keys/{service_principal_key_id}/disable", "Disable key", s.handleDisableSPKey),
		route("DELETE", "/service-principals/{service_principal_id}/keys/{service_principal_key_id}", "Delete key", s.handleDeleteSPKey),
		route("POST", "/service-principals/oauth2/token", "Issue OAuth2 token", s.handleOAuth2Token),

		// Project API Keys
		route("GET", "/compat/api-keys", "List API keys", s.handleListAPIKeys),
		route("POST", "/compat/api-keys", "Create an API key", s.handleCreateAPIKey),
		route("GET", "/compat/api-keys/{apikey_id}", "Get an API key", s.handleReadAPIKey),
		route("PUT", "/compat/api-keys/{apikey_id}", "Update an API key", s.handleUpdateAPIKey),
		route("DELETE", "/compat/api-keys/{apikey_id}", "Delete an API key", s.handleDeleteAPIKey),

		// IAM Roles (read-only)
		route("GET", "/iam-roles", "List IAM roles", s.handleListIAMRoles),
		route("GET", "/iam-roles/{iam_role_id}", "Get an IAM role", s.handleReadIAMRole),

		// ID Roles (read-only)
		route("GET", "/id-roles", "List ID roles", s.handleListIDRoles),
		route("GET", "/id-roles/{id_role_id}", "Get an ID role", s.handleReadIDRole),

		// Organization IAM Policy
		route("GET", "/organization-iam-policy", "Get organization IAM policy", s.handleReadOrgIAMPolicy),
		route("PUT", "/organization-iam-policy", "Update organization IAM policy", s.handleUpdateOrgIAMPolicy),

		// Organization ID Policy
		route("GET", "/organization-id-policy", "Get organization ID policy", s.handleReadOrgIDPolicy),
		route("PUT", "/organization-id-policy", "Update organization ID policy", s.handleUpdateOrgIDPolicy),

		// Organization
		route("GET", "/organization", "Get organization", s.handleReadOrganization),
		route("PUT", "/organization", "Update organization", s.handleUpdateOrganization),

		// Password Policy
		route("GET", "/organization-password-policy", "Get password policy", s.handleReadPasswordPolicy),
		route("PUT", "/organization-password-policy", "Update password policy", s.handleUpdatePasswordPolicy),

		// Auth Conditions
		route("GET", "/organization-auth-conditions", "Get auth conditions", s.handleReadAuthConditions),
		route("PUT", "/organization-auth-conditions", "Update auth conditions", s.handleUpdateAuthConditions),

		// Auth Context
		route("GET", "/auth/context", "Get auth context", s.handleAuthContext),

		// SSO Profiles
		route("GET", "/sso-profiles", "List SSO profiles", s.handleListSSOProfiles),
		route("POST", "/sso-profiles", "Create an SSO profile", s.handleCreateSSOProfile),
		route("GET", "/sso-profiles/{sso_profile_id}", "Get an SSO profile", s.handleReadSSOProfile),
		route("PUT", "/sso-profiles/{sso_profile_id}", "Update an SSO profile", s.handleUpdateSSOProfile),
		route("DELETE", "/sso-profiles/{sso_profile_id}", "Delete an SSO profile", s.handleDeleteSSOProfile),
		route("POST", "/sso-profiles/{sso_profile_id}/assign", "Assign SSO profile", s.handleAssignSSOProfile),
		route("POST", "/sso-profiles/{sso_profile_id}/unassign", "Unassign SSO profile", s.handleUnassignSSOProfile),

		// SCIM Configurations
		route("GET", "/scim-configurations", "List SCIM configurations", s.handleListScimConfigs),
		route("POST", "/scim-configurations", "Create a SCIM configuration", s.handleCreateScimConfig),
		route("GET", "/scim-configurations/{id}", "Get a SCIM configuration", s.handleReadScimConfig),
		route("PUT", "/scim-configurations/{id}", "Update a SCIM configuration", s.handleUpdateScimConfig),
		route("DELETE", "/scim-configurations/{id}", "Delete a SCIM configuration", s.handleDeleteScimConfig),
		route("POST", "/scim-configurations/{id}/regenerate-token", "Regenerate SCIM token", s.handleRegenerateScimToken),

		// Service Policy
		route("POST", "/enable-service-policy", "Enable service policy", s.handleEnableServicePolicy),
		route("POST", "/disable-service-policy", "Disable service policy", s.handleDisableServicePolicy),
		route("GET", "/service-policy-status", "Get service policy status", s.handleServicePolicyStatus),
		route("GET", "/organization-service-policy", "Get organization service policy", s.handleReadOrgServicePolicy),
		route("PUT", "/organization-service-policy", "Update organization service policy", s.handleUpdateOrgServicePolicy),
		route("GET", "/service-policy-rule-templates", "List service policy rule templates", s.handleServicePolicyRuleTemplates),
	}
	return append(table, core.SpecViolationRoutes(s.respValidator)...)
}
