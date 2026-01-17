# Exemplo de Plugin Python

Este diretório contém um exemplo de plugin escrito em Python que injeta informações de contexto dinâmicas.

## Como usar

1.  Certifique-se de que o script é executável:
    ```bash
    chmod +x yby-plugin-context-py
    ```

2.  Este plugin requer Python 3 instalado no ambiente.

3.  Para testar manualmente:
    ```bash
    # Testar hook manifest
    echo '{"hook": "manifest"}' | ./yby-plugin-context-py

    # Testar hook context
    echo '{"hook": "context", "context": {}}' | ./yby-plugin-context-py
    ```

## O que ele faz

Este plugin demonstra como usar Python para lógica mais complexa, injetando as seguintes variáveis no contexto de template:
*   `GENERATION_TIMESTAMP`: Data/hora ISO atual.
*   `OPERATOR_USER`: Usuário do sistema executando o comando.
*   `PYTHON_VERSION`: Versão do interpretador Python.
