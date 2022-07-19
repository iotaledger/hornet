---
description: This section describes the configuration parameters and their types for your HORNET node.
image: /img/Banner/banner_hornet_configuration.png
keywords:
- IOTA Node 
- HORNET Node
- Configuration
- JSON
- Customize
- Config
- reference
---


# Configuration

![HORNET Node Configuration](/img/Banner/banner_hornet_configuration.png)

HORNET uses a JSON standard format as a config file. If you are unsure about JSON syntax, you can find more information in the [official JSON specs](https://www.json.org).

You can change the path of the config file by using the `-c` or `--config` argument while executing `hornet` executable.

For example:
```bash
hornet -c config_example.json
```

You can always get the most up-to-date description of the config parameters by running:

```bash
hornet -h --full
```

