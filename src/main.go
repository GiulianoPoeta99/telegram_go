package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"regexp"
	"strconv"
	"strings"

	"github.com/GiulianoPoeta99/telegram_go.git/src/IA"
	"github.com/GiulianoPoeta99/telegram_go.git/src/db"
	stock "github.com/GiulianoPoeta99/telegram_go.git/src/models/producto"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/joho/godotenv"
)

// Video flipendo : https://www.youtube.com/watch?v=h2AIlBsMkxo

func main() {
	err := godotenv.Load()
	if err != nil {
		log.Fatal("Error al cargar archivo .env")
	}

	telegramBotToken := os.Getenv("TELEGRAM_BOT_TOKEN")
	cohereApiKey := os.Getenv("COHERE_API_KEY")
	databaseURL := os.Getenv("DATABASE_URL")

	if telegramBotToken == "" || cohereApiKey == "" || databaseURL == "" {
		log.Fatal("Asegúrate de establecer TELEGRAM_BOT_TOKEN, COHERE_API_KEY y DATABASE_URL en tu entorno.")
	}

	conn := db.ConnectToDB()
	defer conn.Close(context.Background())

	bot, err := tgbotapi.NewBotAPI(telegramBotToken)
	if err != nil {
		log.Panic(err.Error())
	}

	bot.Debug = true
	log.Printf("Bot autorizado en cuenta %s", bot.Self.UserName)

	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60

	updates := bot.GetUpdatesChan(u)

	synonyms := map[string]string{
		"agregame": "agregar",
		"añadime":  "agregar",
		"añadir":   "agregar",
		"sumar":    "agregar",
		"quitar":   "quitar",
		"eliminar": "quitar",
		"borrar":   "quitar",
	}

	for update := range updates {
		if update.Message == nil {
			continue
		}

		userMessage := update.Message.Text
		userMessageLower := strings.ToLower(userMessage)
		re := regexp.MustCompile(`(?i)(agregar|quitar)\s*(\d+)?\s*(.*)`)
		matches := re.FindStringSubmatch(userMessageLower)

		userID := update.Message.From.ID

		if strings.ToLower(userMessage) == "enviar archivo stock" {
			fileName, err := stock.GenerarArchivoStock(conn, userID)
			if err != nil {
				bot.Send(tgbotapi.NewMessage(update.Message.Chat.ID, "Hubo un error al generar el archivo del stock."))
				continue
			}

			// Abrir el archivo para enviarlo
			file, err := os.Open(fileName)
			if err != nil {
				bot.Send(tgbotapi.NewMessage(update.Message.Chat.ID, "Hubo un error al abrir el archivo."))
				continue
			}

			// Enviar el archivo al usuario
			msg := tgbotapi.NewDocument(update.Message.Chat.ID, tgbotapi.FileReader{
				Name:   fileName,
				Reader: file,
			})
			if _, err := bot.Send(msg); err != nil {
				bot.Send(tgbotapi.NewMessage(update.Message.Chat.ID, "Hubo un error al enviar el archivo."))
			}

			// Eliminar el archivo después de enviarlo para evitar acumulación
			err = os.Remove(fileName)
			if err != nil {
				log.Printf("Error al eliminar el archivo: %v", err)
			}

			continue
		}

		if userMessage == "flipo" {
			// Ruta a la imagen específica
			imagePath := "src/assets/flipo.png"

			// Abrir la imagen
			file, err := os.Open(imagePath)
			if err != nil {
				log.Printf("Error al abrir la imagen: %v", err)
				continue
			}

			// Enviar la imagen al usuario
			msg := tgbotapi.NewPhoto(update.Message.Chat.ID, tgbotapi.FileReader{
				Name:   "flipo.png",
				Reader: file,
			})

			if _, err := bot.Send(msg); err != nil {
				bot.Send(tgbotapi.NewMessage(update.Message.Chat.ID, "Hubo un error al enviar la imagen."))
				log.Printf("Error al enviar la imagen: %v", err)
			}

			// Cerrar el archivo explícitamente después de usarlo
			file.Close()

			continue
		}

		if len(matches) > 0 {
			action := matches[1]
			quantityStr := matches[2]
			product := matches[3]

			if standardAction, exists := synonyms[action]; exists {
				action = standardAction
			}

			if action == "agregar" {
				quantity := 1
				if quantityStr != "" {
					quantity, err = strconv.Atoi(quantityStr)
					if err != nil {
						bot.Send(tgbotapi.NewMessage(update.Message.Chat.ID, "Por favor, proporciona una cantidad válida."))
						continue
					}
				}

				err := stock.AgregarAlStock(conn, userID, product, quantity)
				if err != nil {
					bot.Send(tgbotapi.NewMessage(update.Message.Chat.ID, "Hubo un error al agregar al stock."))
				} else {
					bot.Send(tgbotapi.NewMessage(update.Message.Chat.ID, fmt.Sprintf("Se ha agregado %d %s al stock.", quantity, product)))
				}
			} else if action == "quitar" {
				// Implementar lógica para quitar productos
			}

		} else {
			// Aquí se puede usar Cohere para responder, pero limitando la respuesta al stock
			coherePrompt := fmt.Sprintf("El usuario dice: '%s'. Responde como un bot que le maneja un stock de productos unico a ese usuario especifico. Debes dar respuestas cortas y concisas", userMessage)
			cohereResponse := IA.GetCohereResponse(coherePrompt, cohereApiKey)
			bot.Send(tgbotapi.NewMessage(update.Message.Chat.ID, cohereResponse))
		}
	}
}
