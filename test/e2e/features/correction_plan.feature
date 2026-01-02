Feature: Plano de Correção e Validação de Fluxos Críticos
  Para garantir a robustez do yby-cli
  Como um desenvolvedor ou operador
  Eu quero validar fluxos offline, monorepo e bootstrap sem .env

  Scenario: Fluxo Offline Dev (sem token e repo externo)
    Given que eu estou em um diretório limpo
    When eu executo o comando "yby init --topology single --workflow essential --env dev --project-name test-offline --offline"
    Then o comando deve finalizar com sucesso
    And o arquivo ".yby/environments.yaml" deve existir
    And o arquivo "config/values-local.yaml" deve existir
    And a saída deve conter "Projeto inicializado com sucesso"

  Scenario: Suporte a Monorepo (infra subdir)
    Given que eu estou em um diretório limpo
    When eu executo o comando "yby init --target-dir infra --topology standard --workflow essential --project-name mono-test --offline"
    Then o comando deve finalizar com sucesso
    And eu entro no diretório "infra"
    And eu executo o comando "yby dev"
    Then o comando deve validar os parâmetros
    And a saída deve conter "Contexto Ativo: local"

  Scenario: Bootstrap VPS sem .env (flags explicitas)
    Given que eu estou em um diretório limpo
    When eu executo o comando "yby bootstrap vps --host 127.0.0.1 --user root"
    Then o comando deve validar os parâmetros
    And a saída deve conter "Yby Bootstrap - Provisionamento de VPS"
    And a saída deve conter "root@127.0.0.1"
