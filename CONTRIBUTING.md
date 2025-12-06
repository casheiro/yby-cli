# Contribuindo para o Yby CLI

Obrigado por considerar contribuir para o Yby!

## Como começar

1.  Faça um **Fork** do projeto.
2.  Crie uma **Branch** para sua feature (`git checkout -b feature/MinhaFeature`).
3.  Implemente suas mudanças.
4.  Faça o **Commit** das suas mudanças (`git commit -m 'feat: Adiciona MinhaFeature'`).
5.  Faça o **Push** para a Branch (`git push origin feature/MinhaFeature`).
6.  Abra um **Pull Request**.

## Desenvolvimento Local

Você precisará do [Go](https://go.dev/doc/install) instalado (versão 1.22+).

```bash
# Clone o repositório
git clone https://github.com/casheiro/yby-cli.git
cd yby-cli

# Instale dependências
go mod tidy

# Rode o projeto
go run main.go
```

## Padrões de Commit

Seguimos a convenção do [Conventional Commits](https://www.conventionalcommits.org/).

- `feat`: Uma nova funcionalidade.
- `fix`: Correção de bug.
- `docs`: Apenas documentação.
- `style`: Formatação, ponto e vírgula faltando, etc.
- `refactor`: Refatoração de código em produção.

Exemplo: `feat: adiciona suporte a login via token`
