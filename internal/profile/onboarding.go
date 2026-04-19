package profile

// OnboardingQuestions returns questions for the given onboarding phase and LLM
// instructions for conducting the interview conversationally.
//
// Phase 1 — Basics: home, travel frequency, companions, kids, loyalty.
// Phase 2 — Travel Style: accommodation, budget, transport, remote work, days.
// Phase 3 — Deep Patterns: favourite destinations, neighbourhoods, properties, food, hacks, lounges.
// Phase 4 — Specifics: companion details, wishlist, avoidances, languages/cities, motivation.
//
// Questions already answerable from the profile are skipped. An empty question
// list signals the phase is complete (profile is comprehensive enough).
func OnboardingQuestions(phase int, prof *TravelProfile, answers map[string]string) ([]Question, string) {
	if prof == nil {
		prof = &TravelProfile{}
	}
	if answers == nil {
		answers = map[string]string{}
	}

	instructions := "Ask these questions conversationally, not as a form. Follow up on interesting answers. " +
		"Save responses to the profile using update_preferences or add_booking as appropriate. " +
		"If the traveller has already answered something implicitly, skip it."

	switch phase {
	case 1:
		return phase1Questions(prof, answers), instructions
	case 2:
		return phase2Questions(prof, answers), instructions
	case 3:
		return phase3Questions(prof, answers), instructions
	case 4:
		return phase4Questions(prof, answers), instructions
	default:
		return nil, instructions
	}
}

// phase1Questions returns Phase 1 (Basics) questions, skipping those already
// answered by the profile.
func phase1Questions(prof *TravelProfile, answers map[string]string) []Question {
	var questions []Question

	// Home airport — skip if profile has detected home airports.
	if len(prof.HomeDetected) == 0 && answers["home"] == "" {
		questions = append(questions, Question{
			Key:      "home",
			Text:     "Where are you based? Which city or airport do you usually fly from?",
			Type:     "text",
			Required: true,
		})
	}

	// Travel frequency — no profile signal; always ask if not answered.
	if answers["travel_frequency"] == "" {
		questions = append(questions, Question{
			Key:      "travel_frequency",
			Text:     "How often do you travel? Monthly, quarterly, a few times a year, or rarely?",
			Type:     "choice",
			Options:  []string{"monthly", "quarterly", "yearly", "rarely"},
			Required: false,
		})
	}

	// Travel companions — skip if profile shows consistent solo pattern or has
	// enough bookings to infer (we keep it simple: ask unless answered).
	if answers["travel_companions"] == "" {
		questions = append(questions, Question{
			Key:      "travel_companions",
			Text:     "Who do you usually travel with?",
			Type:     "choice",
			Options:  []string{"solo", "partner", "family", "friends", "mix"},
			Required: false,
		})
	}

	// Kids — only relevant if not solo; skip if already answered.
	companionAnswer := answers["travel_companions"]
	alreadySolo := companionAnswer == "solo"
	if !alreadySolo && answers["kids"] == "" {
		questions = append(questions, Question{
			Key:      "kids",
			Text:     "Do you have children who travel with you?",
			Type:     "choice",
			Options:  []string{"yes", "no", "sometimes"},
			Required: false,
		})
	}

	// Loyalty memberships — no profile signal for this; always ask if not answered.
	if answers["loyalty"] == "" {
		questions = append(questions, Question{
			Key:      "loyalty",
			Text:     "Any frequent flyer or hotel loyalty memberships? (e.g. Finnair Plus, Marriott Bonvoy)",
			Type:     "text",
			Required: false,
		})
	}

	return questions
}

// phase2Questions returns Phase 2 (Travel Style) questions, skipping those
// already covered by the profile.
func phase2Questions(prof *TravelProfile, answers map[string]string) []Question {
	var questions []Question

	// Accommodation type — skip if profile has a preferred type already.
	if prof.PreferredType == "" && answers["accom_type"] == "" {
		questions = append(questions, Question{
			Key:      "accom_type",
			Text:     "Are you more of an apartment or hotel person? What matters most — laundry, kitchen, WiFi, location, or breakfast?",
			Type:     "text",
			Required: false,
		})
	}

	// Budget / nightly rate — skip if profile has avg nightly rate.
	if prof.AvgNightlyRate == 0 && answers["nightly_budget"] == "" {
		q := Question{
			Key:      "nightly_budget",
			Text:     "What's your typical nightly accommodation budget? Do you splurge or go smart?",
			Type:     "text",
			Required: false,
		}
		if prof.BudgetTier != "" {
			q.Default = prof.BudgetTier
		}
		questions = append(questions, q)
	}

	// Ground transport — skip if profile shows non-flight ground modes.
	hasGroundData := len(prof.TopGroundModes) > 0
	if !hasGroundData && answers["transport_modes"] == "" {
		questions = append(questions, Question{
			Key:      "transport_modes",
			Text:     "Do you stick to flights, or do you also take trains, buses, or ferries when travelling?",
			Type:     "multi_choice",
			Options:  []string{"flights_only", "trains", "buses", "ferries", "mix"},
			Required: false,
		})
	}

	// Remote work — no profile signal; ask if not answered.
	if answers["remote_work"] == "" {
		questions = append(questions, Question{
			Key:      "remote_work",
			Text:     "Do you work remotely while travelling?",
			Type:     "choice",
			Options:  []string{"yes_always", "sometimes", "no"},
			Required: false,
		})
	}

	// Preferred travel days — skip if profile has departure day data.
	if len(prof.PreferredDays) == 0 && answers["travel_days"] == "" {
		questions = append(questions, Question{
			Key:      "travel_days",
			Text:     "Do you prefer to travel on weekdays, weekends, or are you flexible?",
			Type:     "choice",
			Options:  []string{"weekdays", "weekends", "flexible"},
			Required: false,
		})
	}

	return questions
}

// phase3Questions returns Phase 3 (Deep Patterns) questions, skipping those
// already covered by the profile.
func phase3Questions(prof *TravelProfile, answers map[string]string) []Question {
	var questions []Question

	// Favourite destinations — skip if profile has top destinations.
	if len(prof.TopDestinations) == 0 && answers["favourite_destinations"] == "" {
		questions = append(questions, Question{
			Key:      "favourite_destinations",
			Text:     "Any cities or places you keep coming back to? What draws you there?",
			Type:     "text",
			Required: false,
		})
	}

	// Favourite neighbourhoods — complements destinations; always ask if not answered.
	if answers["favourite_neighbourhoods"] == "" {
		questions = append(questions, Question{
			Key:      "favourite_neighbourhoods",
			Text:     "In the cities you love, are there particular neighbourhoods or areas you gravitate to?",
			Type:     "text",
			Required: false,
		})
	}

	// Favourite properties — skip if profile has top hotel chains with meaningful data.
	hasHotelData := len(prof.TopHotelChains) > 0 && prof.TopHotelChains[0].Nights >= 3
	if !hasHotelData && answers["favourite_properties"] == "" {
		questions = append(questions, Question{
			Key:      "favourite_properties",
			Text:     "Any hotels, apartments, or properties you've stayed at and loved?",
			Type:     "text",
			Required: false,
		})
	}

	// Food style — no profile signal; ask if not answered.
	if answers["food_style"] == "" {
		questions = append(questions, Question{
			Key:      "food_style",
			Text:     "What's your food style when travelling? Any favourite restaurants or cuisines you seek out?",
			Type:     "text",
			Required: false,
		})
	}

	// Travel hacks — no profile signal; ask if not answered.
	if answers["travel_hacks"] == "" {
		questions = append(questions, Question{
			Key:      "travel_hacks",
			Text:     "Any travel hacks you rely on? (cheap fares + status tricks, bus vs train, early check-in strategies, etc.)",
			Type:     "text",
			Required: false,
		})
	}

	// Lounge preferences — skip if profile has airline alliance (proxy for lounge access).
	if prof.PreferredAlliance == "" && answers["lounges"] == "" {
		questions = append(questions, Question{
			Key:      "lounges",
			Text:     "Do you use airport lounges? Any preferred ones at your home airport?",
			Type:     "text",
			Required: false,
		})
	}

	return questions
}

// phase4Questions returns Phase 4 (Specifics) questions, skipping those
// already covered by the profile or answers.
func phase4Questions(prof *TravelProfile, answers map[string]string) []Question {
	var questions []Question

	// Companion details — ask if they travel with others (inferred from phase 1).
	companionAnswer := answers["travel_companions"]
	isSolo := companionAnswer == "solo"
	if !isSolo && answers["companion_details"] == "" {
		questions = append(questions, Question{
			Key:      "companion_details",
			Text:     "Tell me about your travel companion(s) — names, any preferences or needs I should know about?",
			Type:     "text",
			Required: false,
		})
	}

	// Wishlist destinations — no profile signal; always ask if not answered.
	if answers["wishlist"] == "" {
		questions = append(questions, Question{
			Key:      "wishlist",
			Text:     "Any destinations on your bucket list — places you've been meaning to visit?",
			Type:     "text",
			Required: false,
		})
	}

	// Avoidances — airlines, destinations, or experiences to skip.
	if answers["avoidances"] == "" {
		questions = append(questions, Question{
			Key:      "avoidances",
			Text:     "Anything you try to avoid? Certain airlines, airports, destinations, or types of accommodation?",
			Type:     "text",
			Required: false,
		})
	}

	// Languages and cities lived in — adds context for local recommendations.
	if answers["languages_cities"] == "" {
		questions = append(questions, Question{
			Key:      "languages_cities",
			Text:     "What languages do you speak? And have you lived in any cities other than your current home?",
			Type:     "text",
			Required: false,
		})
	}

	// Travel motivation — mountains, beaches, culture, food, etc.
	if answers["motivation"] == "" {
		questions = append(questions, Question{
			Key:      "motivation",
			Text:     "What draws you to a destination — mountains, beaches, culture, food scenes, nightlife, history?",
			Type:     "multi_choice",
			Options:  []string{"mountains", "beaches", "culture", "food", "nightlife", "history", "nature", "architecture"},
			Required: false,
		})
	}

	return questions
}
