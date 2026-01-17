# Exemplo de Plugin Shell

Este diretório contém um exemplo simples de plugin escrito em Bash.

## Como usar

1.  Certifique-se de que o script é executável:
    ```bash
    chmod +x yby-plugin-hello-sh
    ```

2.  Para testar manualmente, você pode enviar JSON pelo STDIN:
    ```bash
    # Testar hook manifest
    echo '{"hook": "manifest"}' | ./yby-plugin-hello-sh

    # Testar hook context
    echo '{"hook": "context", "context": {}}' | ./yby-plugin-hello-sh
    ```
