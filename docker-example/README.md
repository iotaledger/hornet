Create a file named `.env` containing

```
ACME_EMAIL=your-email@example.com

HORNET_HOST=node.your-domain.com

DASHBOARD_HOST=dashboard.your-domain.com
DASHBOARD_PASSWORD=a5c5c6949e5259b6f74b08019da0b54b056473d2ed4712d8590682e6bd46876b
DASHBOARD_SALT=b5769c198c45b84bf502ed0dde3b698eb885a527dca5bd5b0cd015992157cc79

GRAFANA_HOST=grafana.your-domain.com
```

You can generate your own dashboard password and salt by running:
`docker-compose run hornet tool pwd-hash`

Change the values accordingly, then run `docker-compose up -d`

You can check the logs using `docker-compose logs`
