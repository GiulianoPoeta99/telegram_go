package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/jackc/pgx/v4"
	"github.com/joho/godotenv"
)

// Estructura para la respuesta de Cohere
type CohereResponse struct {
	Generations []struct {
		Text string `json:"text"`
	} `json:"generations"`
}

type Producto struct {
	UserID   int64  `json:"user_id"`
	Producto string `json:"producto"`
	Cantidad int    `json:"cantidad"`
}

// Función para establecer conexión con la BDD
func connectToDB() *pgx.Conn {
	log.Printf("Conectando a la base de datos: %s", os.Getenv("DATABASE_URL"))

	config, err := pgx.ParseConfig(os.Getenv("DATABASE_URL"))
	if err != nil {
		log.Fatalf("No se pudo parsear la URL de la base de datos: %v", err)
	}
	conn, err := pgx.ConnectConfig(context.Background(), config)
	if err != nil {
		log.Fatalf("No se pudo conectar a la base de datos: %v", err)
	}
	return conn
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

func getCohereResponse(prompt, apiKey string) string {
	client := &http.Client{
		Timeout: 60 * time.Second,
	}

	// Modificar el prompt para limitar la respuesta al contexto del stock
	requestBody, err := json.Marshal(map[string]interface{}{
		"model":      "command-xlarge-nightly",
		"prompt":     prompt,
		"max_tokens": 100,
	})
	if err != nil {
		log.Printf("Error al crear el cuerpo de la petición: %v", err)
		return "Hubo un error al procesar tu solicitud."
	}

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

	if resp.StatusCode != http.StatusOK {
		body, _ := ioutil.ReadAll(resp.Body)
		log.Printf("Error en la respuesta de Cohere: %s", string(body))
		return "Error al recibir una respuesta válida de Cohere."
	}

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

	conn := connectToDB()
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
			cohereResponse := getCohereResponse(coherePrompt, cohereApiKey)
			bot.Send(tgbotapi.NewMessage(update.Message.Chat.ID, cohereResponse))
		}
	}
}
