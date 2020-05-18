import * as React from 'react';
import Container from "react-bootstrap/Container";
import Row from "react-bootstrap/Row";
import Col from "react-bootstrap/Col";
import NodeStore from "app/stores/NodeStore";
import {inject, observer} from "mobx-react";
import Card from "react-bootstrap/Card";
import {Line} from "react-chartjs-2";
import {defaultChartOptions} from "app/misc/Chart";
import {If} from "tsx-control-statements/components";
import Badge from "react-bootstrap/Badge";
import * as style from '../../assets/main.css';

interface Props {
    nodeStore?: NodeStore;
}

const lineChartOptions = Object.assign({
    scales: {
        xAxes: [{
            ticks: {
                autoSkip: true,
                maxTicksLimit: 8,
                fontSize: 8,
            },
            gridLines: {
                display: false
            }
        }],
        yAxes: [{
            gridLines: {
                display: false
            },
            ticks: {
                maxTicksLimit: 4,
                suggestedMin: 0,
                beginAtZero: true,
            },
        }],
    },
}, defaultChartOptions);

const reqLineChartOptions = Object.assign({
    scales: {
        xAxes: [{
            ticks: {
                autoSkip: true,
                maxTicksLimit: 8,
                fontSize: 8,
            },
            gridLines: {
                display: false
            }
        }],
        yAxes: [{
            gridLines: {
                display: false
            },
            ticks: {
                callback: function (value, index, values) {
                    return Math.abs(value);
                },
                maxTicksLimit: 4,
                suggestedMin: 0,
                beginAtZero: true,
            },
        }],
    },
    tooltips: {
        callbacks: {
            label: function (tooltipItem, data) {
                let label = data.datasets[tooltipItem.datasetIndex].label;
                return `${label} ${Math.abs(tooltipItem.value)}`;
            }
        }
    }
}, defaultChartOptions);

const cacheLineChartOpts = Object.assign({}, {
    scales: {
        xAxes: [{
            ticks: {
                autoSkip: true,
                maxTicksLimit: 8,
                fontSize: 8,
                minRotation: 0,
                maxRotation: 0,
            },
            showXLabels: 10,
            gridLines: {
                display: false
            }
        }],
        yAxes: [{
            gridLines: {
                display: false
            },
            ticks: {
                fontSize: 10,
                maxTicksLimit: 4,
                suggestedMin: 0,
                beginAtZero: true,
                suggestedMax: 100,
                callback: function (value, index, values) {
                    return `${value}%`;
                }
            },
        }],
    },
    tooltips: {
        callbacks: {
            label: function (tooltipItem, data) {
                return `Hit Rate: ${tooltipItem.value}%`;
            }
        }
    }
}, defaultChartOptions);

const dbSizeLineChartOpts = Object.assign({}, {
    scales: {
        xAxes: [{
            ticks: {
                autoSkip: true,
                maxTicksLimit: 8,
                fontSize: 8,
                minRotation: 0,
                maxRotation: 0,
            },
            showXLabels: 10,
            gridLines: {
                display: false
            }
        }],
        yAxes: [{
            gridLines: {
                display: false
            },
            ticks: {
                fontSize: 10,
                maxTicksLimit: 4,
                suggestedMin: 0,
                beginAtZero: true,
                suggestedMax: 100,
                callback: function (value, index, values) {
                    return `${value} MB`;
                }
            },
        }],
    },
    tooltips: {
        callbacks: {
            label: function (tooltipItem, data) {
                let label = data.datasets[tooltipItem.datasetIndex].label;
                return `${label}: ${tooltipItem.value} MB`;
            }
        }
    }
}, defaultChartOptions);

@inject("nodeStore")
@observer
export class Misc extends React.Component<Props, any> {
    render() {
        return (
            <Container fluid>
                <h3>Misc</h3>
                <Row className={"mb-3"}>
                    <Col>
                        <Card>
                            <Card.Body>
                                <Card.Title>Tip-Selection Performance</Card.Title>
                                <div className={style.hornetChart}>
                                    <Line data={this.props.nodeStore.tipSelSeries}
                                      options={lineChartOptions}/>
                                </div>
                                <div className={style.hornetChart}>
                                    <Line data={this.props.nodeStore.tipSelCacheSeries}
                                      options={cacheLineChartOpts}/>
                                </div>
                            </Card.Body>
                        </Card>
                    </Col>
                </Row>
                <Row className={"mb-3"}>
                    <Col>
                        <Card>
                            <Card.Body>
                                <Card.Title>Request Queue</Card.Title>
                                <div className={style.hornetChart}>
                                    <Line data={this.props.nodeStore.reqQSizeSeries}
                                        options={lineChartOptions}/>
                                </div>
                            </Card.Body>
                        </Card>
                    </Col>
                </Row>
                <Row className={"mb-3"}>
                    <Col>
                        <Card>
                            <Card.Body>
                                <Card.Title>Server Metrics</Card.Title>
                                <div className={style.hornetChart}>
                                    <Line data={this.props.nodeStore.serverMetricsSeries}
                                        options={lineChartOptions}/>
                                </div>
                            </Card.Body>
                        </Card>
                    </Col>
                </Row>
                <Row className={"mb-3"}>
                    <Col>
                        <Card>
                            <Card.Body>
                                <Card.Title>Cache Sizes</Card.Title>
                                <small>
                                    The cache size shrinks whenever an eviction happens.
                                    Note that the sizes are sampled only every second, so you won't necessarily
                                    see the cache hitting its capacity.
                                </small>
                                <div className={style.hornetChart}>
                                    <Line data={this.props.nodeStore.cacheMetricsSeries}
                                        options={reqLineChartOptions}/>
                                </div>
                            </Card.Body>
                        </Card>
                    </Col>
                </Row>
                <Row className={"mb-3"}>
                    <Col>
                        <Card>
                            <Card.Body>
                                <Card.Title>Requests</Card.Title>
                                <div className={style.hornetChart}>
                                    <Line data={this.props.nodeStore.stingReqs}
                                        options={reqLineChartOptions}/>
                                </div>
                            </Card.Body>
                        </Card>
                    </Col>
                </Row>
                <Row className={"mb-3"}>
                    <Col>
                        <Card>
                            <Card.Body>
                                <Card.Title>Database</Card.Title>
                                <If condition={!!this.props.nodeStore.last_dbsize_metric.values}>
                                    <Container className={"d-flex justify-content-between align-items-center"}>
                                        <small>
                                            Size: {((this.props.nodeStore.last_dbsize_metric.keys + this.props.nodeStore.last_dbsize_metric.values) / 1024 / 1024).toFixed(2)} MB
                                            <If condition={this.props.nodeStore.lastDatabaseCleanupDuration > 0}>
                                                <br/>
                                                {"Last GC: "} {this.props.nodeStore.lastDatabaseCleanupEnd} {". Took: "}{this.props.nodeStore.lastDatabaseCleanupDuration}{" seconds."}
                                                <br/>
                                            </If>
                                        </small>
                                        <If condition={this.props.nodeStore.isRunningDatabaseCleanup}>
                                            <Badge variant="danger">GC running</Badge>
                                        </If>
                                    </Container>
                                </If>
                                <div className={style.hornetChart}>
                                    <Line data={this.props.nodeStore.dbSizeSeries}
                                        options={dbSizeLineChartOpts}/>
                                </div>
                            </Card.Body>
                        </Card>
                    </Col>
                </Row>
            </Container>
        );
    }
}
