// Copyright 2016 David Lavieri.  All rights reserved.
// Use of this source code is governed by a MIT License
// License that can be found in the LICENSE file.

package main

import (
	"net/http"
	"strconv"
	"time"

	"golang.org/x/crypto/bcrypt"
)

func userUpdateHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	params, err := paramsFromContext(ctx, nil)
	token, err := jwtFromContext(ctx, err)
	claims, err := claimsFromToken(token, err)

	if err != nil {
		jsonErrorEncode(w, http.StatusForbidden, nil, nil)
	}

	if !isJSONContentType(r) {
		jsonErrorEncode(w, http.StatusBadRequest, errJSONContentType, errJSONContentType)
		return
	}

	// get UserID from request url
	userID, err := strconv.ParseInt(params.ByName("id"), 10, 64)
	if err != nil || userID == 0 {
		jsonErrorEncode(w, http.StatusBadRequest, nil, nil)
		return
	}

	// check if user has permission to update other users or only itself
	canUpdate := isAllowedScope("user:update", claims.Scope) || claims.User.ID == userID
	if !canUpdate {
		jsonErrorEncode(w, http.StatusForbidden, nil, nil)
		return
	}

	// start update user
	response := map[string]interface{}{}
	request := struct {
		User
		Password string `json:"password"`
	}{}

	// decode request body
	if err = jsonDecode(r.Body, &request); err != nil {
		jsonErrorEncode(w, http.StatusBadRequest, errMalformedJSON, err)
		return
	}

	// set data into user struct
	user := &User{
		ID:        userID,
		Name:      request.Name,
		Email:     request.Email,
		UpdatedAt: time.Now(),
	}

	if request.Password != "" {
		var pwd []byte
		pwd, err = bcrypt.GenerateFromPassword([]byte(request.Password), 10)

		if err != nil {
			jsonErrorEncode(w, http.StatusInternalServerError, err, err)
			return
		}

		user.Password = string(pwd)
	}

	// validate user is correctly formed for updated
	if err = user.validateDB(postgres); err != nil {
		if err == errUniqueConstraintViolationDB {
			jsonErrorEncode(w, http.StatusInternalServerError, errUniqueConstraintViolationDB, err)
		} else {
			jsonErrorEncode(w, http.StatusInternalServerError, nil, err)
		}

		return
	}

	// update user
	rows, err := user.updateDB(postgres)
	if err != nil {
		jsonErrorEncode(w, http.StatusInternalServerError, nil, err)
		return
	} else if rows == 0 {
		jsonErrorEncode(w, http.StatusNotFound, nil, nil)
		return
	}

	// get newly updated user
	finalUser, err := user.getByID(user.ID, postgres)
	if err != nil {
		jsonErrorEncode(w, http.StatusInternalServerError, nil, err)
		return
	}

	// set user into response
	response["user"] = finalUser
	response["message"] = map[string]string{
		"type":  "SUCCESS",
		"title": "User successfully updated",
	}

	// set response header
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	// write reponse
	if err := jsonEncode(w, response); err != nil {
		jsonErrorEncode(w, http.StatusInternalServerError, errMalformedJSON, err)
		return
	}

}
