Create a file named `.env` containing

```
ACME_EMAIL=your-email@example.com

HORNET_HOST=node.your-domain.com

DASHBOARD_HOST=dashboard.your-domain.com
DASHBOARD_PASSWORD=0000000000000000000000000000000000000000000000000000000000000000
DASHBOARD_SALT=0000000000000000000000000000000000000000000000000000000000000000

GRAFANA_HOST=grafana.your-domain.com
```

Run `./prepare_docker.sh` to create the `data` folder with correct permissions.

You can generate your own dashboard password and salt by running:
`docker-compose run hornet tool pwd-hash`

Change the values accordingly, then run `docker-compose up -d`

You can check the logs using `docker-compose logs`

You will be able to access your node under:
https://node.your-domain.com/api/v2/info

Your dashboard under:
https://dashboard.your-domain.com

And grafana under:
https://grafana.your-domain.com

**Warning:** the initial grafana credentials are admin/admin, so be sure to log in once to change them.
