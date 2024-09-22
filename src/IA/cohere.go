package IA

import (
	"bytes"
	"encoding/json"
	"io/ioutil"
	"log"
	"net/http"
	"time"
)

// Estructura para la respuesta de Cohere
type CohereResponse struct {
	Generations []struct {
		Text string `json:"text"`
	} `json:"generations"`
}

func GetCohereResponse(prompt, apiKey string) string {
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
