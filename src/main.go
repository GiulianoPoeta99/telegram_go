package main

import (
	"encoding/json"
	"log"
	"os"
	"time"

	"github.com/go-resty/resty/v2"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/joho/godotenv"
)

// Estructura para la respuesta de OpenAI
type OpenAIResponse struct {
	Choices []struct {
		Text string `json:"text"`
	} `json:"choices"`
}

// Función para conectarse a la API de OpenAI y obtener una respuesta
func getOpenAIResponse(prompt, apiKey string) string {
	client := resty.New()
	client.SetTimeout(60 * time.Second)

	// Cuerpo de la petición a OpenAI
	requestBody := map[string]interface{}{
		"model": "gpt-3.5-turbo",
		"messages": []map[string]string{
			{"role": "user", "content": prompt},
		},
	}

	// Hacer la solicitud HTTP a OpenAI
	resp, err := client.R().
		SetHeader("Content-Type", "application/json").
		SetHeader("Authorization", "Bearer "+apiKey).
		SetBody(requestBody).
		Post("https://api.openai.com/v1/chat/completions")

	if err != nil {
		log.Printf("Error en la solicitud a OpenAI: %v", err.Error())
		return "Hubo un error al procesar tu solicitud."
	}

	// Verificar si la respuesta no es 200
	if resp.StatusCode() != 200 {
		log.Printf("Error en la respuesta de OpenAI: %s", resp.String())
		return "Error al recibir una respuesta válida de OpenAI."
	}

	// Imprimir la respuesta completa para depuración
	log.Println("Respuesta completa de OpenAI:", resp.String())

	// Procesar la respuesta JSON
	var openAIResponse OpenAIResponse
	err = json.Unmarshal(resp.Body(), &openAIResponse)
	if err != nil {
		log.Printf("Error al procesar el JSON: %v", err)
		return "Hubo un error al entender la respuesta de OpenAI."
	}

	if len(openAIResponse.Choices) > 0 {
		return openAIResponse.Choices[0].Text
	}

	return "No recibí ninguna respuesta de OpenAI."
}

func main() {
	err := godotenv.Load()
	if err != nil {
		log.Fatal("Error al cargar archivo .env")
	}

	// Cargar las variables de entorno
	telegramBotToken := os.Getenv("TELEGRAM_BOT_TOKEN")
	openAIApiKey := os.Getenv("OPENAI_API_KEY")

	if telegramBotToken == "" || openAIApiKey == "" {
		log.Fatal("Asegúrate de establecer TELEGRAM_BOT_TOKEN y OPENAI_API_KEY en tu entorno.")
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

		// Obtener respuesta de OpenAI
		aiResponse := getOpenAIResponse(userMessage, openAIApiKey)

		// Enviar la respuesta al usuario en Telegram
		reply := tgbotapi.NewMessage(update.Message.Chat.ID, aiResponse)
		bot.Send(reply)
	}
}
