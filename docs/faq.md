# FAQ

### What is HORNET?

HORNET is a community driven IOTA fullnode. It is written in Go which makes it lightweight and fast.

---

### Does HORNET run on the Mainnet?

Yes, HORNET [was released in mid 2020](https://blog.iota.org/hornet-0-4-0-release-summary-2163ca444196/) and replaced the Java implementation called IRI.

---

### Can I run HORNET on a Raspberry Pi?

Yes, you can run HORNET on a Raspberry Pi 4B with an external SSD. But we recommend to run HORNET on a more powerful device.

---

### I have difficulties setting up HORNET. Where can I get help?

Our community loves helping you. Just ask your questions in the `#hornet` channel on the [official IOTA Discord Server](https://discord.iota.org/)

---

### What are the spammer settings for?

You can enable the integrated spammer by adding `"Spammer"` to `"enableplugins"` (comma separated).
These settings are available:

```json
"spammer": {
    "address": "HORNET99INTEGRATED99SPAMMER999999999999999999999999999999999999999999999999999999",
    "depth": 3,
    "message": "Spamming with HORNET tipselect",
    "tag": "HORNET99INTEGRATED99SPAMMER",
    "tpsratelimit": 0.1,
    "workers": 1
  }
```

- `address`: The address you want to use (you don't have to own it as it's zero value spam)
- `depth`: Depth for tip selection. Set it to `3` if you don't know what this is for.
- `message`: The message you want to send with your spam (keep it short)
- `tag`: The tag you want to use (can not be longer than 27 trytes [A-Z, 9])
- `tpsratelimit`: Defines how many transactions (TX) the spammer should try to send (e.g. `0.1` stands for 0.1 TX per second --> 1 TX every 10 seconds. **NOTE:** the maximum `tpsratelimit` is limited by your used hardware. Start with a lower value and slightly increase it, watch your CPU usage.
- `workers`: Number of workers the spammer spins up --> Number of CPU cores you want to use (you should not use all available cores)

---

### Can I contribute?

Of course, you are very welcome! Just send a PR or offer your help in the `#hornet` channel on the [official IOTA Discord Server](https://discord.iota.org/)

---

### I found a bug, what should I do?

Please open a [new issue](https://github.com/gohornet/hornet/issues/new?assignees=&labels=bug&template=bug_report.md&title=). We'll have a look at your bug report as soon as possible.

---

### I'm missing feature xyz. Can you add it?

Please open a [new feature request](https://github.com/gohornet/hornet/issues/new?assignees=&labels=feature&template=feature_request.md&title=). We cannot assure that the feature will actually be implemented.
Pull requests are very welcome!