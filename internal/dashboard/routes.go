package dashboard

func (s *Server) registerRoutes() {
	s.router.Get("/", s.handleHome)
	s.router.Get("/messages", s.handleMessages)
	s.router.Get("/crons", s.handleCrons)
	s.router.Get("/crons/new", s.handleCronForm)
	s.router.Post("/crons", s.handleCronCreate)
	s.router.Get("/crons/{id}", s.handleCronDetail)
	s.router.Get("/crons/{id}/edit", s.handleCronEditForm)
	s.router.Post("/crons/{id}", s.handleCronUpdate)
	s.router.Delete("/crons/{id}", s.handleCronDelete)
	s.router.Post("/crons/{id}/toggle", s.handleCronToggle)
	s.router.Post("/crons/{id}/run", s.handleCronRunNow)
	s.router.Get("/soul", s.handleSoul)
	s.router.Post("/soul", s.handleSoulSave)
	s.router.Get("/activity/stream", s.handleSSE)
}
