package routes

import (
	"fmt"
	"net/http"
	"project/handlers"
)

// HomeRoute: 루트 URL 핸들러
func HomeRoute(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "You called HomeRoute")
}

// TaskRoute: /task 요청 처리 핸들러
func TaskRoute(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "You called TaskRoute\n")

	// 요청 처리 로직 위임
	handlers.HandleTask(w, r)
}
