package main

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
)

func (srv *MyApi) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-type", "application/json")
	var err error
	switch r.URL.Path {
	case "/user/profile":
		err = srv.handlerProfile(w, r)
	case "/user/create":
		err = srv.handlerCreate(w, r)
	default:
		err = ApiError{http.StatusNotFound, errors.New("unknown method")}
	}
	if err != nil {
		aerr, ok := err.(ApiError)
		if !ok {
			http.Error(w, "{\"error\":\""+err.Error()+"\"}", http.StatusInternalServerError)
			return
		}
		w.WriteHeader(aerr.HTTPStatus)
		w.Write([]byte("{\"error\":\"" + aerr.Error() + "\"}"))
		return
	}
	w.WriteHeader(http.StatusOK)
}

// Profile MyApi handler
func (srv *MyApi) handlerProfile(w http.ResponseWriter, r *http.Request) error {
	var loginStr string
	switch r.Method {
	case "GET":
		loginKeys, ok := r.URL.Query()["login"]
		if ok && len(loginKeys) > 0 {
			loginStr = loginKeys[0]
		}
	case "POST":
		loginStr = r.FormValue("login")
	}
	login := loginStr
	if login == "" {
		return ApiError{http.StatusBadRequest, errors.New("login must be not empty")}
	}
	params := ProfileParams{
		Login: login,
	}
	ctx := context.Background()
	res, err := srv.Profile(ctx, params)
	if err != nil {
		return err
	}
	resp := map[string]interface{}{
		"error":    "",
		"response": res,
	}
	json.NewEncoder(w).Encode(resp)
	return nil
}

// Create MyApi handler
func (srv *MyApi) handlerCreate(w http.ResponseWriter, r *http.Request) error {
	if r.Method != "POST" {
		return ApiError{http.StatusNotAcceptable, errors.New("bad method")}
	}
	if r.Header.Get("X-Auth") != "100500" {
		return ApiError{http.StatusForbidden, errors.New("unauthorized")}
	}
	var loginStr string
	switch r.Method {
	case "GET":
		loginKeys, ok := r.URL.Query()["login"]
		if ok && len(loginKeys) > 0 {
			loginStr = loginKeys[0]
		}
	case "POST":
		loginStr = r.FormValue("login")
	}
	login := loginStr
	if login == "" {
		return ApiError{http.StatusBadRequest, errors.New("login must be not empty")}
	}

	if len(login) < 10 {
		return ApiError{http.StatusBadRequest, errors.New("login len must be >= 10")}
	}
	var nameStr string
	switch r.Method {
	case "GET":
		nameKeys, ok := r.URL.Query()["full_name"]
		if ok && len(nameKeys) > 0 {
			nameStr = nameKeys[0]
		}
	case "POST":
		nameStr = r.FormValue("full_name")
	}
	name := nameStr

	var statusStr string
	switch r.Method {
	case "GET":
		statusKeys, ok := r.URL.Query()["status"]
		if ok && len(statusKeys) > 0 {
			statusStr = statusKeys[0]
		}
	case "POST":
		statusStr = r.FormValue("status")
	}
	status := statusStr
	if status != "" {
		if status != "user" && status != "moderator" && status != "admin" {
			return ApiError{http.StatusBadRequest, errors.New("status must be one of [user, moderator, admin]")}
		}
	}
	if status == "" {
		status = "user"
	}
	var ageStr string
	switch r.Method {
	case "GET":
		ageKeys, ok := r.URL.Query()["age"]
		if ok && len(ageKeys) > 0 {
			ageStr = ageKeys[0]
		}
	case "POST":
		ageStr = r.FormValue("age")
	}
	ageInt, err := strconv.Atoi(ageStr)
	if err != nil {
		return ApiError{http.StatusBadRequest, errors.New("age must be int")}
	}
	age := ageInt
	if age < 0 {
		return ApiError{http.StatusBadRequest, errors.New("age must be >= 0")}
	}
	if age > 128 {
		return ApiError{http.StatusBadRequest, errors.New("age must be <= 128")}
	}
	params := CreateParams{
		Login:  login,
		Name:   name,
		Status: status,
		Age:    age,
	}
	ctx := context.Background()
	res, err := srv.Create(ctx, params)
	if err != nil {
		return err
	}
	resp := map[string]interface{}{
		"error":    "",
		"response": res,
	}
	json.NewEncoder(w).Encode(resp)
	return nil
}

func (srv *OtherApi) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-type", "application/json")
	var err error
	switch r.URL.Path {
	case "/user/create":
		err = srv.handlerCreate(w, r)
	default:
		err = ApiError{http.StatusNotFound, errors.New("unknown method")}
	}
	if err != nil {
		aerr, ok := err.(ApiError)
		if !ok {
			http.Error(w, "{\"error\":\""+err.Error()+"\"}", http.StatusInternalServerError)
			return
		}
		w.WriteHeader(aerr.HTTPStatus)
		w.Write([]byte("{\"error\":\"" + aerr.Error() + "\"}"))
		return
	}
	w.WriteHeader(http.StatusOK)
}

// Create OtherApi handler
func (srv *OtherApi) handlerCreate(w http.ResponseWriter, r *http.Request) error {
	if r.Method != "POST" {
		return ApiError{http.StatusNotAcceptable, errors.New("bad method")}
	}
	if r.Header.Get("X-Auth") != "100500" {
		return ApiError{http.StatusForbidden, errors.New("unauthorized")}
	}
	var usernameStr string
	switch r.Method {
	case "GET":
		usernameKeys, ok := r.URL.Query()["username"]
		if ok && len(usernameKeys) > 0 {
			usernameStr = usernameKeys[0]
		}
	case "POST":
		usernameStr = r.FormValue("username")
	}
	username := usernameStr
	if username == "" {
		return ApiError{http.StatusBadRequest, errors.New("username must be not empty")}
	}

	if len(username) < 3 {
		return ApiError{http.StatusBadRequest, errors.New("username len must be >= 3")}
	}
	var nameStr string
	switch r.Method {
	case "GET":
		nameKeys, ok := r.URL.Query()["account_name"]
		if ok && len(nameKeys) > 0 {
			nameStr = nameKeys[0]
		}
	case "POST":
		nameStr = r.FormValue("account_name")
	}
	name := nameStr

	var classStr string
	switch r.Method {
	case "GET":
		classKeys, ok := r.URL.Query()["class"]
		if ok && len(classKeys) > 0 {
			classStr = classKeys[0]
		}
	case "POST":
		classStr = r.FormValue("class")
	}
	class := classStr
	if class != "" {
		if class != "warrior" && class != "sorcerer" && class != "rouge" {
			return ApiError{http.StatusBadRequest, errors.New("class must be one of [warrior, sorcerer, rouge]")}
		}
	}
	if class == "" {
		class = "warrior"
	}
	var levelStr string
	switch r.Method {
	case "GET":
		levelKeys, ok := r.URL.Query()["level"]
		if ok && len(levelKeys) > 0 {
			levelStr = levelKeys[0]
		}
	case "POST":
		levelStr = r.FormValue("level")
	}
	levelInt, err := strconv.Atoi(levelStr)
	if err != nil {
		return ApiError{http.StatusBadRequest, errors.New("level must be int")}
	}
	level := levelInt
	if level < 1 {
		return ApiError{http.StatusBadRequest, errors.New("level must be >= 1")}
	}
	if level > 50 {
		return ApiError{http.StatusBadRequest, errors.New("level must be <= 50")}
	}
	params := OtherCreateParams{
		Username: username,
		Name:     name,
		Class:    class,
		Level:    level,
	}
	ctx := context.Background()
	res, err := srv.Create(ctx, params)
	if err != nil {
		return err
	}
	resp := map[string]interface{}{
		"error":    "",
		"response": res,
	}
	json.NewEncoder(w).Encode(resp)
	return nil
}
