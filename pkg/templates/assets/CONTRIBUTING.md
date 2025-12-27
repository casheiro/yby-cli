# 🤝 Como Contribuir

Obrigado por considerar contribuir para o **Yby**! Este guia define os padrões para garantir que o projeto continue limpo, seguro e fácil de manter, especialmente com o auxílio da automação de CI/CD.

## 🛠️ Fluxo de Desenvolvimento e Integração

O fluxo de trabalho segue um modelo baseado em branches, focado na automação e validação contínua.

1.  **Fork & Clone**: Faça um fork deste repositório e clone localmente.
2.  **Branch de Feature**: Crie uma branch para sua feature ou correção (`git checkout -b feature/minha-melhoria`).
3.  **Ambiente Local**:
    -   Use `make setup-local` para instalar as ferramentas essenciais (kubectl, helm, k3d).
    -   **Recomendado:** Instale o [direnv](https://direnv.net/) e execute `direnv allow` na raiz do projeto. Isso configurará automaticamente o `KUBECONFIG` isolado (`./.kube/config`) toda vez que você entrar na pasta, evitando conflitos com seu cluster global.
    -   Use `make dev` para subir um cluster k3d isolado e testar suas mudanças.
4.  **Push e PR para `develop`**:
    -   Faça `push` da sua branch `feature/*` para o seu repositório remoto.
    -   Este `push` irá acionar o `feature-pipeline.yaml`, que rodará validações estáticas (`make validate`).
    -   **Automaticamente**, um Pull Request (PR) será aberto da sua branch `feature/*` para a branch `develop`.
5.  **Revisão e Merge na `develop`**:
    -   O PR para `develop` deve ser revisado e aprovado por outro membro do time.
    -   Uma vez aprovado, ele pode ser mergeado na `develop`.

## 🚀 Fluxo de Release Automatizado (Visão do Contribuidor)

Após o merge na `develop`, um processo automatizado é acionado:

1.  **Staging da Release:** O merge na `develop` dispara a automação que calcula a próxima versão, cria uma branch `release/vX.Y.Z` (a partir da `main`) e abre um PR de **"staging"** da `develop` para `release/vX.Y.Z`. Você será notificado no commit de merge da `develop` com o link para este PR.
2.  **Merge de Staging:** O PR de "staging" é revisado e mergeado na branch `release/vX.Y.Z`. Este é o ponto onde os testes E2E seriam validados se tivéssemos essa etapa aqui, mas não temos.
3.  **PR para `main`:** Este merge aciona a criação da Tag, da Release no GitHub, e abre o PR final da `release/vX.Y.Z` para a `main`.
4.  **Validação para `main`:** Este PR final para a `main` aciona o `pr-main-checks.yaml`, que roda a validação de commit e os testes E2E.

## 📝 Padrões de Commit

É **obrigatório** seguir a convenção [Conventional Commits](https://www.conventionalcommits.org/). Nossas pipelines de CI/CD aplicam estas regras e falharão se o formato não for respeitado.

-   `feat: nova funcionalidade` (ex: `feat: adiciona suporte a backup s3`)
-   `fix: correção de bug` (ex: `fix: corrige erro de login em dispositivos moveis`)
-   `docs: alteração em documentação`
-   `chore: tarefas de manutenção (deps, scripts)`
-   `refactor: melhoria de código sem alterar comportamento`
-   `test: adição ou correção de testes`
-   `build: alterações no sistema de build ou dependências externas`
-   `ci: alterações nos arquivos de CI/CD`
-   `revert: reverte um commit anterior`
-   `release: criação de uma nova versão (uso exclusivo da automação)`

Exemplo:
```bash
git commit -m "feat(auth): adiciona autenticacao via oauth2"
```

## 🧪 Testes

Temos dois níveis de testes no projeto:

1.  **Validação Estática (`make validate`)**:
    -   **O que faz:** Realiza linting e validação de templates dos charts Helm.
    -   **Quando rodar:** Sempre **antes de commitar**.
    -   **Onde roda na CI:** No `feature-pipeline.yaml` (em cada `push` da feature) e no `release-automation.yaml` (na validação inicial da branch de release).
    -   **Comando:**
        ```bash
        make validate
        ```

2.  **Testes End-to-End (E2E) (`make ci-test`)**:
    -   **O que faz:** Sobe um cluster Kubernetes local (`k3d`), instala toda a plataforma e roda workflows de teste para verificar a integração dos componentes.
    -   **Quando rodar na CI:** **Apenas** no Pull Request final da `release/*` para a `main` (`pr-main-checks.yaml`).
    -   **Como rodar localmente:**
        ```bash
        make ci-test
        ```
        *(Este comando irá subir o ambiente com `make dev`, rodar os testes e depois fazer `make clean` automaticamente.)*

## 📄 Documentação

-   Se sua mudança altera o comportamento do sistema, atualize o `README.md` ou os arquivos em `docs/`.
-   Mantenha a documentação concisa e direta.

## 🐛 Reportando Bugs

Abra uma Issue no GitHub com:
1.  Descrição clara e concisa do problema.
2.  Passos detalhados para reproduzir o bug.
3.  Logs relevantes (use `kubectl logs ...` ou logs da CI/CD).
4.  Ambiente onde o bug foi observado (Local/VPS, versão do K8s, etc.).

---

**Dúvidas?** Abra uma Discussion no repositório ou contate o time de plataforma.