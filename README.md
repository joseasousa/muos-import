# SaveSync para muOS e KNULLI

Protótipo para sincronizar saves do RetroArch no RG40XX V. Usa um cartão de boot para cada firmware e um SD2 exFAT compartilhado. A interface exibe o resultado da sincronização e registra os detalhes em `sync.log`.

## Compilar e configurar

1. Gere os pacotes com Go 1.23 ou superior, um compilador C para Linux ARM64 e o SDL2 de desenvolvimento para esse alvo. Em um Linux ARM64 com `pkg-config`, os headers e a biblioteca de desenvolvimento instalados, rode `./package.sh`. Para compilar de outra arquitetura, aponte `CC` para o compilador ARM64 e configure `PKG_CONFIG_SYSROOT_DIR` e `PKG_CONFIG_LIBDIR` para que o `pkg-config` encontre o SDL2 do sysroot ARM64. Use um sysroot compatível com os dois firmwares. O binário usa o SDL2 fornecido pelo firmware em execução.
2. Copie `dist/muos/MUOS/application/SaveSync` para `MUOS/application/` no armazenamento do muOS. No armazenamento do KNULLI, copie a pasta `SaveSync` e o arquivo `SaveSync.sh` de `dist/knulli/roms/ports/` para `roms/ports/`. Use um SD2 de teste e crie manualmente um arquivo vazio chamado `.savesync-card` na raiz dele.
3. Abra o SaveSync pelo menu do firmware. Use o direcional para navegar, **A** para confirmar e **B** para voltar. Na primeira abertura, selecione a raiz do SD2, a biblioteca de ROMs e as pastas efetivas de saves dos cores que usa. O SaveSync espera o save na raiz configurada do core, com o mesmo nome-base da ROM ou do ZIP. Use Gambatte para GB/GBC e mGBA para GBA. Salve a configuração e selecione **Verificar configuração**.
4. Feche o RetroArch e selecione **Sincronizar**. Consulte o resultado na tela e os detalhes em `sync.log`, ao lado de `config.json`. Desligue o aparelho para alternar o SD1 e rode o SaveSync no outro firmware antes de jogar.

Para configurar por arquivo, copie `config.example.json` para `config.json` ao lado do executável e preencha os caminhos absolutos medidos no aparelho. Rode `./savesync -config config.json -action check` para verificar a configuração sem iniciar a sincronização. Para desenvolver apenas a CLI, use `go build .` e `go test ./...`.

## Backups e conflitos

Os dados ficam em `SaveSync/` no SD2. O manifesto mantém a origem e a data dos arquivos. Em alterações concorrentes, vence a data mais recente. Empates exigem escolha. Mantenha o relógio do aparelho acertado. A partir de um terminal no aparelho:

```sh
./savesync -config config.json -action list
./savesync -config config.json -action restore -key 'CHAVE' -hash HASH -confirm
```

## Save states e validação física

Deixe os states desativados até validar ambos os builds com conteúdo descartável. Para ensaios, configure `States` no core, a mesma identificação de combinação em `StateProfile` nos dois lados e `BuildFiles` com caminhos e hashes SHA-256 dos executáveis e cores locais. Em `ApprovedStates`, inclua os hashes dos states existentes cuja origem foi confirmada. O protótipo reconhece `.state`, `.state.auto` e slots de 1 a 999. Se o RetroArch estiver configurado para carregar o slot automático, o jogo poderá abrir diretamente naquele estado.

A consulta de backups e conflitos e a restauração usam a CLI. Falta validar no RG40XX V a abertura da interface, o vídeo SDL2, os botões, os caminhos e o retorno aos menus dos firmwares. Valide também os builds, o carregamento de states e a recuperação após interrupção em SD2 exFAT.
