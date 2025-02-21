package handlers

import (
	"html/template"
	"net/http"

	"github.com/thespecialone1/go-rammerly/auth"
	"github.com/thespecialone1/go-rammerly/models"
)

// tmpl is our parsed HTML template.
var tmpl *template.Template

// InitTemplates parses the HTML template.
func InitTemplates() {
	tmpl = template.Must(template.ParseFiles("templates/index.html"))
}

// HandleHome renders the home page and shows user info if available.
func HandleHome(w http.ResponseWriter, r *http.Request) {
	session, err := auth.GetSession(r)
	var userInfo *models.User
	if err == nil {
		if uid, ok := session.Values["user_id"]; ok {
			// Retrieve the basic user info from the session.
			userInfo = &models.User{
				ID:      uid.(int64), // Assuming your user ID is int64.
				Email:   session.Values["user_email"].(string),
				Name:    session.Values["user_name"].(string),
				Picture: session.Values["picture"].(string),
			}
		}
	}

	data := models.PageData{
		CurrentPage: "home",
		User:        userInfo,
	}
	tmpl.Execute(w, data)
}
