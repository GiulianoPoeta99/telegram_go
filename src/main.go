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
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/jackc/pgx/v4"
	"github.com/joho/godotenv"
)

type Producto struct {
	UserID   int64  `json:"user_id"`
	Producto string `json:"producto"`
	Cantidad int    `json:"cantidad"`
}

func agregarAlStock(conn *pgx.Conn, userID int64, producto string, cantidad int) error {
	var currentQuantity int
	err := conn.QueryRow(context.Background(),
		"SELECT cantidad FROM stock WHERE user_id = $1 AND producto = $2", userID, producto).Scan(&currentQuantity)

	if err != nil {
		if err == pgx.ErrNoRows {
			_, err = conn.Exec(context.Background(),
				"INSERT INTO stock (user_id, producto, cantidad) VALUES ($1, $2, $3)", userID, producto, cantidad)
			if err != nil {
				log.Printf("Error al agregar nuevo producto: %v", err)
				return fmt.Errorf("error al agregar al stock: %v", err)
			}
			log.Printf("Producto '%s' agregado al stock por el usuario %d.", producto, userID)
		} else {
			log.Printf("Error al consultar el stock: %v", err)
			return fmt.Errorf("error al consultar el stock: %v", err)
		}
	} else {
		newQuantity := currentQuantity + cantidad
		_, err = conn.Exec(context.Background(),
			"UPDATE stock SET cantidad = $1 WHERE user_id = $2 AND producto = $3", newQuantity, userID, producto)
		if err != nil {
			log.Printf("Error al actualizar el producto: %v", err)
			return fmt.Errorf("error al actualizar el stock: %v", err)
		}
		log.Printf("Producto '%s' actualizado. Nueva cantidad: %d para el usuario %d.", producto, newQuantity, userID)
	}
	return nil
}

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

				err := agregarAlStock(conn, userID, product, quantity)
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
