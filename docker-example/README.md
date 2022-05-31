Create a file named `.env` containing

```
ACME_EMAIL=your-email@example.com

HORNET_HOST=node.your-domain.com

DASHBOARD_HOST=dashboard.your-domain.com
DASHBOARD_PASSWORD=0000000000000000000000000000000000000000000000000000000000000000
DASHBOARD_SALT=0000000000000000000000000000000000000000000000000000000000000000

GRAFANA_HOST=grafana.your-domain.com
```

You can generate your own dashboard password and salt by running:
`docker-compose run hornet tool pwd-hash`

Change the values accordingly, then run `docker-compose up -d`

You can check the logs using `docker-compose logs`
