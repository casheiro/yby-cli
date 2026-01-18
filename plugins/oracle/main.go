package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/casheiro/yby-cli/plugins/oracle/internal/rag"
	"github.com/spf13/cobra"
)

type PluginManifest struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Version     string   `json:"version"`
	Hooks       []string `json:"hooks"`
}

type PluginRequest struct {
	Hook string `json:"hook"`
}

type PluginResponse struct {
	Data interface{} `json:"data"`
}

// Global Indexer for MVP
var indexer = rag.NewSimpleIndexer()

func main() {
	var rootCmd = &cobra.Command{
		Use:   "oracle",
		Short: "Yby Oracle - Knowledge & RAG Plugin",
		Run: func(cmd *cobra.Command, args []string) {
			cmd.Help()
		},
	}

	var indexCmd = &cobra.Command{
		Use:   "index [dir]",
		Short: "Indexa a documentação local",
		Args:  cobra.MaximumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			dir := "./docs"
			if len(args) > 0 {
				dir = args[0]
			}
			if err := indexer.Index(context.Background(), dir); err != nil {
				fmt.Printf("Erro ao indexar: %v\n", err)
			}
		},
	}

	var askCmd = &cobra.Command{
		Use:   "ask [question]",
		Short: "Faz uma pergunta ao Oracle",
		Args:  cobra.MinimumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			question := args[0]
			// Busca ingênua no índice em memória (que estará vazio pois não persiste entre execuções no MVP)
			// Para demonstrar, vamos indexar on-the-fly se estiver vazio e existir pasta docs, ou apenas simular.

			// Simulação de persistencia para MVP: Indexer roda na memória.
			// Se rodarmos 'oracle ask' sem 'oracle index' antes no mesmo processo, falha.
			// Como são processos distintos, precisamos de persistência.
			// Para o MVP Agora: vamos assumir que ele indexa "./docs" antes de responder se quisermos testar real,
			// ou apenas mockamos a resposta.

			// Vamos tentar responder algo.
			fmt.Printf("Oracle: Pesquisando na base de conhecimento por '%s'...\n", question)
			docs, _ := indexer.Search(context.Background(), question)
			if len(docs) > 0 {
				fmt.Println("Found relevant context:")
				for _, d := range docs {
					fmt.Printf("- %s\n", d.Source)
				}
			} else {
				fmt.Println("Nenhuma correspondência exata encontrada no índice da sessão atual.")
			}
		},
	}

	var manifestCmd = &cobra.Command{
		Use:    "manifest",
		Hidden: true,
		Run: func(cmd *cobra.Command, args []string) {
			manifest := PluginManifest{
				Name:        "oracle",
				Description: "Knowledge & RAG Plugin",
				Version:     "0.1.0",
				Hooks:       []string{"command"},
			}
			json.NewEncoder(os.Stdout).Encode(map[string]interface{}{
				"data": manifest,
			})
		},
	}

	rootCmd.AddCommand(indexCmd, askCmd, manifestCmd)

	stat, _ := os.Stdin.Stat()
	if (stat.Mode() & os.ModeCharDevice) == 0 {
		var req PluginRequest
		if err := json.NewDecoder(os.Stdin).Decode(&req); err == nil {
			if req.Hook == "manifest" {
				manifest := PluginManifest{
					Name:        "oracle",
					Description: "Knowledge & RAG Plugin",
					Version:     "0.1.0",
					Hooks:       []string{"command"},
				}
				json.NewEncoder(os.Stdout).Encode(PluginResponse{Data: manifest})
				return
			}
		}
	}

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
