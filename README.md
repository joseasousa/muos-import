# SaveSync para muOS e KNULLI

Protótipo para sincronizar saves do RetroArch no RG40XX V. Usa um cartão de boot para cada firmware e um SD2 exFAT compartilhado. Os launchers de teste escrevem o resultado em `sync.log`.

## Compilar e configurar

1. Rode `go test ./...` e `./package.sh` com Go 1.23 ou superior. O script gera árvores para muOS e KNULLI em `dist/` e compila um executável Linux ARM64.
2. Copie a árvore de cada pacote para o armazenamento correspondente. No muOS, a árvore começa em `MUOS/application`. No KNULLI, começa em `roms/ports`. Use um SD2 de teste e crie um arquivo vazio chamado `.savesync-card` na raiz dele.
3. Ao lado de cada executável, copie `config.example.json` para `config.json`. Em cada firmware, indique o caminho absoluto sob o qual o SD2 está montado em `Card`, a biblioteca local em `ROMs` e as pastas efetivas de saída em `Cores`. Confira o nome e o caminho que o RetroArch realmente usa com um save descartável. O protótipo espera o save na raiz configurada do core, com o mesmo nome-base da ROM ou do ZIP. Use Gambatte para GB/GBC e mGBA para GBA.
4. Feche o RetroArch e abra o launcher. Examine `sync.log`. Desligue o aparelho para alternar o SD1 e rode o launcher no outro firmware antes de jogar.

## Backups e conflitos

Os dados ficam em `SaveSync/` no SD2. O manifesto mantém a origem e a data dos arquivos. Em alterações concorrentes, vence a data mais recente. Empates exigem escolha. Mantenha o relógio do aparelho acertado. A partir de um terminal no aparelho:

```sh
./savesync -config config.json -action list
./savesync -config config.json -action restore -key 'CHAVE' -hash HASH -confirm
```

## Save states e validação física

Deixe os states desativados até validar ambos os builds com conteúdo descartável. Para ensaios, configure `States` no core, a mesma identificação de combinação em `StateProfile` nos dois lados e `BuildFiles` com caminhos e hashes SHA-256 dos executáveis e cores locais. Em `ApprovedStates`, inclua os hashes dos states existentes cuja origem foi confirmada. O protótipo reconhece `.state`, `.state.auto` e slots de 1 a 999. Se o RetroArch estiver configurado para carregar o slot automático, o jogo poderá abrir diretamente naquele estado.

A configuração, a revisão do relatório e a restauração ainda usam arquivos ou terminal. Falta validar os atalhos, caminhos, builds, carregamento dos states e recuperação após interrupção em SD2 exFAT no RG40XX V.
