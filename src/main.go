package main

import (
	"bytes"
	"encoding/json"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/joho/godotenv"
)

// Estructura para la respuesta de Cohere
type CohereResponse struct {
	Generations []struct {
		Text string `json:"text"`
	} `json:"generations"`
}

// Función para conectarse a la API de Cohere y obtener una respuesta
func getCohereResponse(prompt, apiKey string) string {
	client := &http.Client{
		Timeout: 60 * time.Second,
	}

	// Cuerpo de la petición a Cohere
	requestBody, _ := json.Marshal(map[string]interface{}{
		"model":      "command-xlarge-nightly", // Modelo de generación de Cohere
		"prompt":     prompt,
		"max_tokens": 100, // Puedes ajustar el número de tokens según tu necesidad
	})

	// Hacer la solicitud HTTP a Cohere
	req, err := http.NewRequest("POST", "https://api.cohere.ai/v1/generate", bytes.NewBuffer(requestBody))
	if err != nil {
		log.Printf("Error creando la solicitud a Cohere: %v", err.Error())
		return "Hubo un error al procesar tu solicitud."
	}

	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		log.Printf("Error en la solicitud a Cohere: %v", err.Error())
		return "Hubo un error al procesar tu solicitud."
	}
	defer resp.Body.Close()

	// Verificar si la respuesta no es 200
	if resp.StatusCode != 200 {
		body, _ := ioutil.ReadAll(resp.Body)
		log.Printf("Error en la respuesta de Cohere: %s", string(body))
		return "Error al recibir una respuesta válida de Cohere."
	}

	// Procesar la respuesta JSON
	var cohereResponse CohereResponse
	err = json.NewDecoder(resp.Body).Decode(&cohereResponse)
	if err != nil {
		log.Printf("Error al procesar el JSON: %v", err)
		return "Hubo un error al entender la respuesta de Cohere."
	}

	if len(cohereResponse.Generations) > 0 {
		return cohereResponse.Generations[0].Text
	}

	return "No recibí ninguna respuesta de Cohere."
}

func main() {
	// Cargar archivo .env
	err := godotenv.Load()
	if err != nil {
		log.Fatal("Error al cargar archivo .env")
	}

	// Cargar las variables de entorno
	telegramBotToken := os.Getenv("TELEGRAM_BOT_TOKEN")
	cohereApiKey := os.Getenv("COHERE_API_KEY")

	if telegramBotToken == "" || cohereApiKey == "" {
		log.Fatal("Asegúrate de establecer TELEGRAM_BOT_TOKEN y COHERE_API_KEY en tu entorno.")
	}

	// Iniciar el bot de Telegram
	bot, err := tgbotapi.NewBotAPI(telegramBotToken)
	if err != nil {
		log.Panic(err.Error())
	}

	bot.Debug = true
	log.Printf("Bot autorizado en cuenta %s", bot.Self.UserName)

	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60

	updates := bot.GetUpdatesChan(u)

	// Escuchar mensajes de Telegram
	for update := range updates {
		if update.Message == nil {
			continue
		}

		userMessage := update.Message.Text
		log.Printf("[%s] %s", update.Message.From.UserName, userMessage)

		// Obtener respuesta de Cohere
		aiResponse := getCohereResponse(userMessage, cohereApiKey)

		// Enviar la respuesta al usuario en Telegram
		reply := tgbotapi.NewMessage(update.Message.Chat.ID, aiResponse)
		bot.Send(reply)
	}
}
