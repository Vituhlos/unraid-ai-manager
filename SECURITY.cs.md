# Bezpečnostní politika

Unraid AI Manager je privilegovaný lokální automatizační nástroj. Je potřeba s ním zacházet stejně opatrně jako s jakýmkoliv nástrojem, který může upravovat Unraid Docker šablony.

## Podporované verze

Dokud je projekt ve fázi preview `0.y.z`, podporovaná je pouze poslední vydaná verze.

## Bezpečnostní model

AI klient ani MCP server nejsou bezpečnostní hranice.

Bezpečnostní hranice je helper daemon běžící na Unraidu. Ten musí:

- vystavovat jen whitelistované akce;
- odmítat raw shell příkazy;
- vyžadovat přesné hashe plánů pro apply operace;
- vytvářet backupy před XML zápisy;
- zapisovat audit logy;
- volitelně vyžadovat krátkodobé lokální approval tokeny;
- nevystavovat MCP klientům neomezený Docker socket.

## Výchozí síťové nastavení

Helper daemon by měl poslouchat na:

```text
127.0.0.1:37231
```

Doporučený vzdálený přístup je přes SSH tunnel:

```bash
ssh -L 37231:127.0.0.1:37231 root@<unraid-ip>
```

Helper nevystavuj přímo do internetu.

## Rizikové operace

Tyto operace musí zůstat blokované nebo vyžadovat explicitní extra schválení:

- raw shell execution;
- privileged kontejnery;
- mount host filesystemu jako `/`, `/boot`, `/etc`, `/usr`;
- neomezené vystavení `/var/run/docker.sock`;
- mount celého `/mnt` nebo `/mnt/user`;
- host networking;
- libovolné devices a capabilities;
- recreate/start/stop/remove kontejnerů;
- instalace Community Applications;
- instalace pluginů;
- destruktivní diskové, array, VM nebo share operace.

## Hlášení zranitelnosti

Prozatím hlaste zranitelnosti soukromě vlastníkovi repozitáře přes GitHub.

Uveď prosím:

- dotčenou verzi;
- kroky k reprodukci;
- očekávaný dopad;
- jestli problém vyžaduje lokální přístup, LAN přístup nebo remote přístup;
- známý návrh mitigace, pokud existuje.

Nezveřejňuj exploit detaily před vydáním opravené verze.
