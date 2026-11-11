// Package ui is the Spanish catalog for the application chrome.
// Study-language text never comes from here.
package ui

func T(key string) string {
	if v, ok := text[key]; ok {
		return v
	}
	return "??" + key
}

var text = map[string]string{
	"app.name":           "Idiomas",
	"app.eyebrow":        "Práctica de idiomas",
	"nav.activities":     "Actividades",
	"nav.settings":       "Ajustes",
	"activity.translate": "Traducción",

	"lang.eng": "Inglés",
	"lang.rus": "Ruso",

	"prompt.to_eng":   "Traduce al inglés",
	"prompt.to_rus":   "Traduce al ruso",
	"prompt.choose":   "Elige una traducción",
	"prompt.type":     "Escribe la traducción",
	"prompt.submit":   "Corregir",
	"prompt.instant":  "Práctica instantánea",
	"prompt.question": "Pregunta %d de %d",

	"result.exact":    "Correcto",
	"result.partial":  "Casi",
	"result.wrong":    "Incorrecto",
	"result.given":    "Tu respuesta:",
	"result.accepted": "Se aceptaba:",
	"result.points":   "Puntos de esta pregunta:",
	"result.running":  "Respondidas %d de %d · Puntaje %s",

	"summary.title":       "Resultado",
	"summary.details":     "Ver el detalle",
	"summary.again":       "Otra partida",
	"summary.back":        "Volver a practicar",
	"summary.empty":       "Esta partida no tiene respuestas.",
	"summary.instant":     "Práctica instantánea, sin puntaje final.",
	"summary.row_prompt":  "Palabra",
	"summary.row_given":   "Respuesta",
	"summary.row_outcome": "Resultado",
	"summary.row_points":  "Puntos",

	"settings.title": "Ajustes",
	"settings.intro": "Estos ajustes cambian la dificultad, la complejidad y la extensión de Traducción. La siguiente palabra ya los usa.",
	"settings.save":  "Guardar",
	"settings.saved": "Ajustes guardados.",
	"settings.about": "Acerca de",

	"about.tatoeba": "Oraciones de Tatoeba, con licencia CC BY 2.0 FR. Parte del corpus también está en CC0.",

	"empty.title":        "Todavía no hay palabras importadas",
	"empty.body":         "Importa oraciones cortas de Tatoeba en inglés y ruso. El archivo de muestra del repositorio sirve para probar:",
	"empty.command_note": "Para el corpus completo, usa los exportes por idioma de Tatoeba y links.csv con las mismas opciones.",

	"error.same_language": "El idioma del usuario y el idioma objetivo tienen que ser distintos.",
	"error.invalid":       "Revisa los ajustes: hay un valor que no sirve.",
	"error.not_found":     "No hay una partida con ese número.",
	"error.no_items":      "No hay palabras con estos ajustes. Prueba otra banda de dificultad.",
	"error.generic":       "No se pudo completar la acción.",

	"setting.user_lang":           "Idioma del usuario",
	"setting.user_lang.help":      "El idioma que ya conoces, dentro del par de estudio.",
	"setting.target_lang":         "Idioma objetivo",
	"setting.target_lang.help":    "El idioma que estás practicando.",
	"setting.direction":           "Dirección",
	"setting.direction.help":      "Desde tu idioma hacia el objetivo, al revés, o mezclado.",
	"setting.direction.to_target": "Hacia el idioma objetivo",
	"setting.direction.to_user":   "Hacia mi idioma",
	"setting.direction.mixed":     "Mezclado",
	"setting.mode":                "Modo",
	"setting.mode.help":           "Una partida tiene un puntaje final. La práctica instantánea entrega palabras sueltas.",
	"setting.mode.instant":        "Práctica instantánea",
	"setting.mode.quiz":           "Partida",
	"setting.quiz_size":           "Preguntas de la partida",
	"setting.quiz_size.help":      "Cuántas palabras tiene la siguiente partida.",
	"setting.input":               "Cómo responder",
	"setting.input.choice":        "Elegir de un listado",
	"setting.input.type":          "Escribir",
	"setting.option_count":        "Opciones del listado",
	"setting.option_count.help":   "Incluye la respuesta correcta.",
	"setting.tolerance":           "Tolerancia al escribir",
	"setting.tolerance.help":      "Ediciones permitidas. El acierto exacto vale 1 y uno dentro de la tolerancia vale 0,5.",
	"setting.difficulty":          "Dificultad",
	"setting.difficulty.help":     "Según la frecuencia en el corpus importado. 1 es lo más frecuente.",
	"setting.difficulty.any":      "Cualquiera",
	"setting.difficulty.1":        "1 · muy frecuentes",
	"setting.difficulty.2":        "2",
	"setting.difficulty.3":        "3",
	"setting.difficulty.4":        "4",
	"setting.difficulty.5":        "5 · poco frecuentes",
	"setting.distractors":         "Opciones incorrectas",
	"setting.distractors.help":    "El largo parecido usa palabras de una longitud similar a la correcta.",
	"setting.distractors.random":  "Al azar",
	"setting.distractors.length":  "Largo parecido",
	"setting.max_tokens":          "Largo del texto",
	"setting.max_tokens.help":     "En esta versión el máximo es una palabra. El filtro queda listo para frases.",

	"message.no_items_title": "Sin palabras para estos ajustes",
	"message.settings_link":  "Cambiar ajustes",
}
