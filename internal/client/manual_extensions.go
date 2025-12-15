package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/oapi-codegen/runtime"
	openapi_types "github.com/oapi-codegen/runtime/types"
)

// Role defines the model for a Role.
type Role struct {
	Id          openapi_types.UUID `json:"id"`
	Name        string             `json:"name"`
	Description *string            `json:"description,omitempty"`
	Permissions []string           `json:"permissions"`
	CreatedAt   *string            `json:"createdAt,omitempty"`
	UpdatedAt   *string            `json:"updatedAt,omitempty"`
}

// CreateRoleJSONBody defines parameters for CreateRole.
type CreateCustomRoleJSONBody struct {
	Name        string   `json:"name"`
	Description *string  `json:"description,omitempty"`
	Permissions []string `json:"permissions"`
}

// UpdateRoleJSONBody defines parameters for UpdateRole.
type UpdateCustomRoleJSONBody struct {
	Name        *string   `json:"name,omitempty"`
	Description *string   `json:"description,omitempty"`
	Permissions *[]string `json:"permissions,omitempty"`
}

// UserRoleAssignment defines the model for assigning a role to a user.
type UserRoleAssignment struct {
	Id     openapi_types.UUID `json:"id"`
	UserId openapi_types.UUID `json:"userId"`
	RoleId openapi_types.UUID `json:"roleId"`
}

// CreateUserRoleAssignmentJSONBody defines parameters for CreateUserRoleAssignment.
type CreateUserRoleAssignmentJSONBody struct {
	UserId openapi_types.UUID `json:"userId"`
	RoleId openapi_types.UUID `json:"roleId"`
}

// CreateCustomRole creates a new role.
func (c *Client) CreateCustomRole(ctx context.Context, body CreateCustomRoleJSONBody, reqEditors ...RequestEditorFn) (*http.Response, error) {
	req, err := NewCreateCustomRoleRequest(c.Server, body)
	if err != nil {
		return nil, err
	}
	req = req.WithContext(ctx)
	if err := c.applyEditors(ctx, req, reqEditors); err != nil {
		return nil, err
	}
	return c.Client.Do(req)
}

// NewCreateCustomRoleRequest generates requests for CreateCustomRole.
func NewCreateCustomRoleRequest(server string, body CreateCustomRoleJSONBody) (*http.Request, error) {
	var err error
	pathParam0 := server + "/v1/roles"
	serverURL, err := url.Parse(pathParam0)
	if err != nil {
		return nil, err
	}

	operationPath := serverURL.String()
	if operationPath[0] == '/' {
		operationPath = "." + operationPath
	}

	queryURL, err := serverURL.Parse(operationPath)
	if err != nil {
		return nil, err
	}

	var bodyReader io.Reader
	buf, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	bodyReader = bytes.NewReader(buf)

	req, err := http.NewRequest("POST", queryURL.String(), bodyReader)
	if err != nil {
		return nil, err
	}

	req.Header.Add("Content-Type", "application/json")

	return req, nil
}

// GetCustomRole returns a role by ID.
func (c *Client) GetCustomRole(ctx context.Context, roleId openapi_types.UUID, reqEditors ...RequestEditorFn) (*http.Response, error) {
	req, err := NewGetCustomRoleRequest(c.Server, roleId)
	if err != nil {
		return nil, err
	}
	req = req.WithContext(ctx)
	if err := c.applyEditors(ctx, req, reqEditors); err != nil {
		return nil, err
	}
	return c.Client.Do(req)
}

func NewGetCustomRoleRequest(server string, roleId openapi_types.UUID) (*http.Request, error) {
	var err error

	pathParam0, err := runtime.StyleParamWithLocation("simple", false, "roleId", runtime.ParamLocationPath, roleId)
	if err != nil {
		return nil, err
	}

	serverURL, err := url.Parse(server)
	if err != nil {
		return nil, err
	}

	operationPath := fmt.Sprintf("/v1/roles/%s", pathParam0)
	if operationPath[0] == '/' {
		operationPath = "." + operationPath
	}

	queryURL, err := serverURL.Parse(operationPath)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("GET", queryURL.String(), nil)
	if err != nil {
		return nil, err
	}

	return req, nil
}

// UpdateCustomRole updates a role.
func (c *Client) UpdateCustomRole(ctx context.Context, roleId string, body UpdateCustomRoleJSONBody, reqEditors ...RequestEditorFn) (*http.Response, error) {
	req, err := NewUpdateCustomRoleRequest(c.Server, roleId, body)
	if err != nil {
		return nil, err
	}
	req = req.WithContext(ctx)
	if err := c.applyEditors(ctx, req, reqEditors); err != nil {
		return nil, err
	}
	return c.Client.Do(req)
}

func NewUpdateCustomRoleRequest(server string, roleId string, body UpdateCustomRoleJSONBody) (*http.Request, error) {
	var err error

	pathParam0, err := runtime.StyleParamWithLocation("simple", false, "roleId", runtime.ParamLocationPath, roleId)
	if err != nil {
		return nil, err
	}

	serverURL, err := url.Parse(server)
	if err != nil {
		return nil, err
	}

	operationPath := fmt.Sprintf("/v1/roles/%s", pathParam0)
	if operationPath[0] == '/' {
		operationPath = "." + operationPath
	}

	queryURL, err := serverURL.Parse(operationPath)
	if err != nil {
		return nil, err
	}

	var bodyReader io.Reader
	buf, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	bodyReader = bytes.NewReader(buf)

	req, err := http.NewRequest("PATCH", queryURL.String(), bodyReader)
	if err != nil {
		return nil, err
	}

	req.Header.Add("Content-Type", "application/json")

	return req, nil
}

// DeleteCustomRole deletes a role.
func (c *Client) DeleteCustomRole(ctx context.Context, roleId openapi_types.UUID, reqEditors ...RequestEditorFn) (*http.Response, error) {
	req, err := NewDeleteCustomRoleRequest(c.Server, roleId)
	if err != nil {
		return nil, err
	}
	req = req.WithContext(ctx)
	if err := c.applyEditors(ctx, req, reqEditors); err != nil {
		return nil, err
	}
	return c.Client.Do(req)
}

func NewDeleteCustomRoleRequest(server string, roleId openapi_types.UUID) (*http.Request, error) {
	var err error

	pathParam0, err := runtime.StyleParamWithLocation("simple", false, "roleId", runtime.ParamLocationPath, roleId)
	if err != nil {
		return nil, err
	}

	serverURL, err := url.Parse(server)
	if err != nil {
		return nil, err
	}

	operationPath := fmt.Sprintf("/v1/roles/%s", pathParam0)
	if operationPath[0] == '/' {
		operationPath = "." + operationPath
	}

	queryURL, err := serverURL.Parse(operationPath)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("DELETE", queryURL.String(), nil)
	if err != nil {
		return nil, err
	}

	return req, nil
}

// Internal structures for higher level client

type CreateCustomRoleResponse struct {
	Body         []byte
	HTTPResponse *http.Response
	JSON201      *Role
}

func (c *ClientWithResponses) CreateCustomRoleWithResponse(ctx context.Context, body CreateCustomRoleJSONBody, reqEditors ...RequestEditorFn) (*CreateCustomRoleResponse, error) {
	client, ok := c.ClientInterface.(*Client)
	if !ok {
		return nil, fmt.Errorf("ClientWithResponses must wrap *Client")
	}
	rsp, err := client.CreateCustomRole(ctx, body, reqEditors...)
	if err != nil {
		return nil, err
	}
	return ParseCreateCustomRoleResponse(rsp)
}

func ParseCreateCustomRoleResponse(rsp *http.Response) (*CreateCustomRoleResponse, error) {
	bodyBytes, err := io.ReadAll(rsp.Body)
	defer func() { _ = rsp.Body.Close() }()
	if err != nil {
		return nil, err
	}

	response := &CreateCustomRoleResponse{
		Body:         bodyBytes,
		HTTPResponse: rsp,
	}

	switch {
	case strings.Contains(rsp.Header.Get("Content-Type"), "json") && rsp.StatusCode == 201:
		var dest Role
		if err := json.Unmarshal(bodyBytes, &dest); err != nil {
			return nil, err
		}
		response.JSON201 = &dest
	}

	return response, nil
}

type GetCustomRoleResponse struct {
	Body         []byte
	HTTPResponse *http.Response
	JSON200      *Role
}

func (c *ClientWithResponses) GetCustomRoleWithResponse(ctx context.Context, roleId openapi_types.UUID, reqEditors ...RequestEditorFn) (*GetCustomRoleResponse, error) {
	client, ok := c.ClientInterface.(*Client)
	if !ok {
		return nil, fmt.Errorf("ClientWithResponses must wrap *Client")
	}
	rsp, err := client.GetCustomRole(ctx, roleId, reqEditors...)
	if err != nil {
		return nil, err
	}
	return ParseGetCustomRoleResponse(rsp)
}

func ParseGetCustomRoleResponse(rsp *http.Response) (*GetCustomRoleResponse, error) {
	bodyBytes, err := io.ReadAll(rsp.Body)
	defer func() { _ = rsp.Body.Close() }()
	if err != nil {
		return nil, err
	}

	response := &GetCustomRoleResponse{
		Body:         bodyBytes,
		HTTPResponse: rsp,
	}

	switch {
	case strings.Contains(rsp.Header.Get("Content-Type"), "json") && rsp.StatusCode == 200:
		var dest Role
		if err := json.Unmarshal(bodyBytes, &dest); err != nil {
			return nil, err
		}
		response.JSON200 = &dest
	}

	return response, nil
}

type UpdateCustomRoleResponse struct {
	Body         []byte
	HTTPResponse *http.Response
	JSON200      *Role
}

func (c *ClientWithResponses) UpdateCustomRoleWithResponse(ctx context.Context, roleId openapi_types.UUID, body UpdateCustomRoleJSONBody, reqEditors ...RequestEditorFn) (*UpdateCustomRoleResponse, error) {
	client, ok := c.ClientInterface.(*Client)
	if !ok {
		return nil, fmt.Errorf("ClientWithResponses must wrap *Client")
	}
	rsp, err := client.UpdateCustomRole(ctx, roleId.String(), body, reqEditors...)
	if err != nil {
		return nil, err
	}
	return ParseUpdateCustomRoleResponse(rsp)
}

func ParseUpdateCustomRoleResponse(rsp *http.Response) (*UpdateCustomRoleResponse, error) {
	bodyBytes, err := io.ReadAll(rsp.Body)
	defer func() { _ = rsp.Body.Close() }()
	if err != nil {
		return nil, err
	}

	response := &UpdateCustomRoleResponse{
		Body:         bodyBytes,
		HTTPResponse: rsp,
	}

	switch {
	case strings.Contains(rsp.Header.Get("Content-Type"), "json") && rsp.StatusCode == 200:
		var dest Role
		if err := json.Unmarshal(bodyBytes, &dest); err != nil {
			return nil, err
		}
		response.JSON200 = &dest
	}

	return response, nil
}

type DeleteCustomRoleResponse struct {
	Body         []byte
	HTTPResponse *http.Response
}

func (c *ClientWithResponses) DeleteCustomRoleWithResponse(ctx context.Context, roleId openapi_types.UUID, reqEditors ...RequestEditorFn) (*DeleteCustomRoleResponse, error) {
	client, ok := c.ClientInterface.(*Client)
	if !ok {
		return nil, fmt.Errorf("ClientWithResponses must wrap *Client")
	}
	rsp, err := client.DeleteCustomRole(ctx, roleId, reqEditors...)
	if err != nil {
		return nil, err
	}
	return ParseDeleteCustomRoleResponse(rsp)
}

func ParseDeleteCustomRoleResponse(rsp *http.Response) (*DeleteCustomRoleResponse, error) {
	bodyBytes, err := io.ReadAll(rsp.Body)
	defer func() { _ = rsp.Body.Close() }()
	if err != nil {
		return nil, err
	}

	response := &DeleteCustomRoleResponse{
		Body:         bodyBytes,
		HTTPResponse: rsp,
	}

	return response, nil
}

// User Role Assignments

func (c *Client) CreateUserRoleAssignment(ctx context.Context, body CreateUserRoleAssignmentJSONBody, reqEditors ...RequestEditorFn) (*http.Response, error) {
	req, err := NewCreateUserRoleAssignmentRequest(c.Server, body)
	if err != nil {
		return nil, err
	}
	req = req.WithContext(ctx)
	if err := c.applyEditors(ctx, req, reqEditors); err != nil {
		return nil, err
	}
	return c.Client.Do(req)
}

func NewCreateUserRoleAssignmentRequest(server string, body CreateUserRoleAssignmentJSONBody) (*http.Request, error) {
	var err error
	pathParam0 := server + "/v1/user-role-assignments"
	serverURL, err := url.Parse(pathParam0)
	if err != nil {
		return nil, err
	}

	operationPath := serverURL.String()
	if operationPath[0] == '/' {
		operationPath = "." + operationPath
	}

	queryURL, err := serverURL.Parse(operationPath)
	if err != nil {
		return nil, err
	}

	var bodyReader io.Reader
	buf, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	bodyReader = bytes.NewReader(buf)

	req, err := http.NewRequest("POST", queryURL.String(), bodyReader)
	if err != nil {
		return nil, err
	}

	req.Header.Add("Content-Type", "application/json")

	return req, nil
}

func (c *Client) DeleteUserRoleAssignment(ctx context.Context, assignmentId openapi_types.UUID, reqEditors ...RequestEditorFn) (*http.Response, error) {
	req, err := NewDeleteUserRoleAssignmentRequest(c.Server, assignmentId)
	if err != nil {
		return nil, err
	}
	req = req.WithContext(ctx)
	if err := c.applyEditors(ctx, req, reqEditors); err != nil {
		return nil, err
	}
	return c.Client.Do(req)
}

func NewDeleteUserRoleAssignmentRequest(server string, assignmentId openapi_types.UUID) (*http.Request, error) {
	var err error

	pathParam0, err := runtime.StyleParamWithLocation("simple", false, "assignmentId", runtime.ParamLocationPath, assignmentId)
	if err != nil {
		return nil, err
	}

	serverURL, err := url.Parse(server)
	if err != nil {
		return nil, err
	}

	operationPath := fmt.Sprintf("/v1/user-role-assignments/%s", pathParam0)
	if operationPath[0] == '/' {
		operationPath = "." + operationPath
	}

	queryURL, err := serverURL.Parse(operationPath)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("DELETE", queryURL.String(), nil)
	if err != nil {
		return nil, err
	}

	return req, nil
}

type CreateUserRoleAssignmentResponse struct {
	Body         []byte
	HTTPResponse *http.Response
	JSON201      *UserRoleAssignment
}

func (c *ClientWithResponses) CreateUserRoleAssignmentWithResponse(ctx context.Context, body CreateUserRoleAssignmentJSONBody, reqEditors ...RequestEditorFn) (*CreateUserRoleAssignmentResponse, error) {
	client, ok := c.ClientInterface.(*Client)
	if !ok {
		return nil, fmt.Errorf("ClientWithResponses must wrap *Client")
	}
	rsp, err := client.CreateUserRoleAssignment(ctx, body, reqEditors...)
	if err != nil {
		return nil, err
	}
	return ParseCreateUserRoleAssignmentResponse(rsp)
}

func ParseCreateUserRoleAssignmentResponse(rsp *http.Response) (*CreateUserRoleAssignmentResponse, error) {
	bodyBytes, err := io.ReadAll(rsp.Body)
	defer func() { _ = rsp.Body.Close() }()
	if err != nil {
		return nil, err
	}

	response := &CreateUserRoleAssignmentResponse{
		Body:         bodyBytes,
		HTTPResponse: rsp,
	}

	switch {
	case strings.Contains(rsp.Header.Get("Content-Type"), "json") && rsp.StatusCode == 201:
		var dest UserRoleAssignment
		if err := json.Unmarshal(bodyBytes, &dest); err != nil {
			return nil, err
		}
		response.JSON201 = &dest
	}

	return response, nil
}

type DeleteUserRoleAssignmentResponse struct {
	Body         []byte
	HTTPResponse *http.Response
}

func (c *ClientWithResponses) DeleteUserRoleAssignmentWithResponse(ctx context.Context, assignmentId openapi_types.UUID, reqEditors ...RequestEditorFn) (*DeleteUserRoleAssignmentResponse, error) {
	client, ok := c.ClientInterface.(*Client)
	if !ok {
		return nil, fmt.Errorf("ClientWithResponses must wrap *Client")
	}
	rsp, err := client.DeleteUserRoleAssignment(ctx, assignmentId, reqEditors...)
	if err != nil {
		return nil, err
	}
	return ParseDeleteUserRoleAssignmentResponse(rsp)
}

func ParseDeleteUserRoleAssignmentResponse(rsp *http.Response) (*DeleteUserRoleAssignmentResponse, error) {
	bodyBytes, err := io.ReadAll(rsp.Body)
	defer func() { _ = rsp.Body.Close() }()
	if err != nil {
		return nil, err
	}

	response := &DeleteUserRoleAssignmentResponse{
		Body:         bodyBytes,
		HTTPResponse: rsp,
	}

	return response, nil
}

// User methods

type User struct {
	Id            openapi_types.UUID `json:"id"`
	Email         string             `json:"email"`
	Name          string             `json:"name"`
	EmailVerified bool               `json:"emailVerified"`
	Image         *string            `json:"image,omitempty"`
	Role          *string            `json:"role,omitempty"`
	Banned        bool               `json:"banned"`
	BanReason     *string            `json:"banReason,omitempty"`
}

type CreateUserJSONBody struct {
	Email         string  `json:"email"`
	Name          string  `json:"name"`
	EmailVerified bool    `json:"emailVerified"`
	Image         *string `json:"image,omitempty"`
	Role          *string `json:"role,omitempty"`
	Banned        bool    `json:"banned"`
	BanReason     *string `json:"banReason,omitempty"`
}

type UpdateUserJSONBody struct {
	Email         *string `json:"email,omitempty"`
	Name          *string `json:"name,omitempty"`
	EmailVerified *bool   `json:"emailVerified,omitempty"`
	Image         *string `json:"image,omitempty"`
	Role          *string `json:"role,omitempty"`
	Banned        *bool   `json:"banned,omitempty"`
	BanReason     *string `json:"banReason,omitempty"`
}

// CreateUser creates a new user.
func (c *Client) CreateUser(ctx context.Context, body CreateUserJSONBody, reqEditors ...RequestEditorFn) (*User, error) {
	req, err := NewCreateUserRequest(c.Server, body)
	if err != nil {
		return nil, err
	}
	req = req.WithContext(ctx)
	if err := c.applyEditors(ctx, req, reqEditors); err != nil {
		return nil, err
	}
	rsp, err := c.Client.Do(req)
	if err != nil {
		return nil, err
	}

	bodyBytes, err := io.ReadAll(rsp.Body)
	defer func() { _ = rsp.Body.Close() }()
	if err != nil {
		return nil, err
	}

	if rsp.StatusCode != 201 {
		return nil, fmt.Errorf("unexpected status code: %d, body: %s", rsp.StatusCode, string(bodyBytes))
	}

	var dest User
	if err := json.Unmarshal(bodyBytes, &dest); err != nil {
		return nil, err
	}
	return &dest, nil
}

func NewCreateUserRequest(server string, body CreateUserJSONBody) (*http.Request, error) {
	var err error
	pathParam0 := server + "/v1/users"
	serverURL, err := url.Parse(pathParam0)
	if err != nil {
		return nil, err
	}

	operationPath := serverURL.String()
	if operationPath[0] == '/' {
		operationPath = "." + operationPath
	}

	queryURL, err := serverURL.Parse(operationPath)
	if err != nil {
		return nil, err
	}

	var bodyReader io.Reader
	buf, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	bodyReader = bytes.NewReader(buf)

	req, err := http.NewRequest("POST", queryURL.String(), bodyReader)
	if err != nil {
		return nil, err
	}

	req.Header.Add("Content-Type", "application/json")

	return req, nil
}

// GetUser returns a user by ID.
func (c *Client) GetUser(ctx context.Context, userId string, reqEditors ...RequestEditorFn) (*User, error) {
	// userId is string here because resource passed string directly, but client expects UUID usually.
	// However, manual extension GetUser signature in previous step used openapi_types.UUID.
	// But resource_user.go calls it with `data.ID.ValueString()`.
	// So I should adjust signature to accept string or UUID.
	// Since resource_user.go passes string, I'll accept string and parse it here or change resource to parsed UUID.
	// Let's stick to resource doing parsing if possible, or string here.
	// Actually, previously defined GetUser took openapi_types.UUID. Resource called it with ValueString() which is wrong type.
	// So I will change this to take string to match resource usage OR fix resource.
	// Resource usage: `user, err := r.client.GetUser(ctx, data.ID.ValueString())`
	// So I will make it take string for easier usage in this manual extension, or parse it.

	// Wait, standard generated client methods usually take UUID if path param is UUID.
	// I should probably follow that pattern.
	// But to fix compilation quickly, I will accept string and parse internally or just pass string if API takes string.
	// The struct uses openapi_types.UUID.

	// Let's use string for ID in arguments to be flexible with Resource call,
	// unless we want strict typing. Resource passes string.

	req, err := NewGetUserRequest(c.Server, userId)
	if err != nil {
		return nil, err
	}
	req = req.WithContext(ctx)
	if err := c.applyEditors(ctx, req, reqEditors); err != nil {
		return nil, err
	}
	rsp, err := c.Client.Do(req)
	if err != nil {
		return nil, err
	}

	bodyBytes, err := io.ReadAll(rsp.Body)
	defer func() { _ = rsp.Body.Close() }()
	if err != nil {
		return nil, err
	}

	if rsp.StatusCode != 200 {
		return nil, fmt.Errorf("unexpected status code: %d, body: %s", rsp.StatusCode, string(bodyBytes))
	}

	var dest User
	if err := json.Unmarshal(bodyBytes, &dest); err != nil {
		return nil, err
	}
	return &dest, nil
}

func NewGetUserRequest(server string, userId string) (*http.Request, error) {
	var err error
	pathParam0 := userId // Assume simple string concat for now or use runtime.StyleParam

	serverURL, err := url.Parse(server)
	if err != nil {
		return nil, err
	}

	operationPath := fmt.Sprintf("/v1/users/%s", pathParam0)
	if operationPath[0] == '/' {
		operationPath = "." + operationPath
	}

	queryURL, err := serverURL.Parse(operationPath)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("GET", queryURL.String(), nil)
	if err != nil {
		return nil, err
	}

	return req, nil
}

// UpdateUser updates a user.
func (c *Client) UpdateUser(ctx context.Context, userId string, body *User, reqEditors ...RequestEditorFn) (*User, error) {
	// Note: body is *User in resource_user.go usage, but typically should be UpdateUserJSONBody
	// Resource constructs *client.User and passes it.
	// So I'll accept *User and map it to UpdateUserJSONBody or just send it if backend accepts it.
	// I'll map it to UpdateUserJSONBody to be safe.

	reqBody := UpdateUserJSONBody{
		Name:          &body.Name,
		Email:         &body.Email,
		EmailVerified: &body.EmailVerified,
		Image:         body.Image,
		Role:          body.Role,
		Banned:        &body.Banned,
		BanReason:     body.BanReason,
	}

	req, err := NewUpdateUserRequest(c.Server, userId, reqBody)
	if err != nil {
		return nil, err
	}
	req = req.WithContext(ctx)
	if err := c.applyEditors(ctx, req, reqEditors); err != nil {
		return nil, err
	}
	rsp, err := c.Client.Do(req)
	if err != nil {
		return nil, err
	}

	bodyBytes, err := io.ReadAll(rsp.Body)
	defer func() { _ = rsp.Body.Close() }()
	if err != nil {
		return nil, err
	}

	if rsp.StatusCode != 200 {
		return nil, fmt.Errorf("unexpected status code: %d, body: %s", rsp.StatusCode, string(bodyBytes))
	}

	var dest User
	if err := json.Unmarshal(bodyBytes, &dest); err != nil {
		return nil, err
	}
	return &dest, nil
}

func NewUpdateUserRequest(server string, userId string, body UpdateUserJSONBody) (*http.Request, error) {
	var err error

	serverURL, err := url.Parse(server)
	if err != nil {
		return nil, err
	}

	operationPath := fmt.Sprintf("/v1/users/%s", userId)
	if operationPath[0] == '/' {
		operationPath = "." + operationPath
	}

	queryURL, err := serverURL.Parse(operationPath)
	if err != nil {
		return nil, err
	}

	var bodyReader io.Reader
	buf, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	bodyReader = bytes.NewReader(buf)

	req, err := http.NewRequest("PATCH", queryURL.String(), bodyReader)
	if err != nil {
		return nil, err
	}

	req.Header.Add("Content-Type", "application/json")

	return req, nil
}

// DeleteUser deletes a user.
func (c *Client) DeleteUser(ctx context.Context, userId string, reqEditors ...RequestEditorFn) error {
	req, err := NewDeleteUserRequest(c.Server, userId)
	if err != nil {
		return err
	}
	req = req.WithContext(ctx)
	if err := c.applyEditors(ctx, req, reqEditors); err != nil {
		return err
	}
	rsp, err := c.Client.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = rsp.Body.Close() }()

	if rsp.StatusCode != 200 && rsp.StatusCode != 204 {
		bodyBytes, _ := io.ReadAll(rsp.Body)
		return fmt.Errorf("unexpected status code: %d, body: %s", rsp.StatusCode, string(bodyBytes))
	}
	return nil
}

func NewDeleteUserRequest(server string, userId string) (*http.Request, error) {
	var err error

	serverURL, err := url.Parse(server)
	if err != nil {
		return nil, err
	}

	operationPath := fmt.Sprintf("/v1/users/%s", userId)
	if operationPath[0] == '/' {
		operationPath = "." + operationPath
	}

	queryURL, err := serverURL.Parse(operationPath)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("DELETE", queryURL.String(), nil)
	if err != nil {
		return nil, err
	}

	return req, nil
}

type GetUserResponse struct {
	Body         []byte
	HTTPResponse *http.Response
	JSON200      *User
	JSON404      *interface{}
}

// GetUserWithResponse is for DataSource use.
func (c *ClientWithResponses) GetUserWithResponse(ctx context.Context, userId openapi_types.UUID, reqEditors ...RequestEditorFn) (*GetUserResponse, error) {
	// Convert UUID to string for the manual implementation
	// This is a bit of a hack to bridge the gap between generated code and manual methods
	// Note: previous implementation took openapi_types.UUID, but my new GetUser takes string.
	// I should probably make GetUser take string or UUID consistently.
	// But since I'm rewriting the entire block, I'll handle it.

	// Actually, having two GetUsers (Client and ClientWithResponses) with different signatures (string vs UUID) is messy.
	// I'll assume userId is UUID and convert to string.

	client, ok := c.ClientInterface.(*Client)
	if !ok {
		return nil, fmt.Errorf("ClientWithResponses must wrap *Client")
	}

	idStr := userId.String()

	// Let's implement `GetUserWithResponse` properly using basic request building.
	req, err := NewGetUserRequest(client.Server, idStr)
	if err != nil {
		return nil, err
	}
	req = req.WithContext(ctx)
	rspHttp, err := client.Client.Do(req)
	if err != nil {
		return nil, err
	}
	return ParseGetUserResponse(rspHttp)
}

func ParseGetUserResponse(rsp *http.Response) (*GetUserResponse, error) {
	bodyBytes, err := io.ReadAll(rsp.Body)
	defer func() { _ = rsp.Body.Close() }()
	if err != nil {
		return nil, err
	}

	response := &GetUserResponse{
		Body:         bodyBytes,
		HTTPResponse: rsp,
	}

	switch {
	case strings.Contains(rsp.Header.Get("Content-Type"), "json") && rsp.StatusCode == 200:
		var dest User
		if err := json.Unmarshal(bodyBytes, &dest); err != nil {
			return nil, err
		}
		response.JSON200 = &dest
	case rsp.StatusCode == 404:
		// verify logic for 404
		response.JSON404 = new(interface{})
	}

	return response, nil
}
