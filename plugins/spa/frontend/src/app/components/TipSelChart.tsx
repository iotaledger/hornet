import * as React from 'react';
import Card from "react-bootstrap/Card";
import NodeStore from "app/stores/NodeStore";
import {inject, observer} from "mobx-react";
import {Line} from "react-chartjs-2";
import 'chartjs-plugin-streaming';
import {defaultChartOptions} from "app/misc/Chart";

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
                beginAtZero: true,
            },
        }],
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

@inject("nodeStore")
@observer
export default class TipSelChart extends React.Component<Props, any> {
    render() {
        return (
            <Card>
                <Card.Body>
                    <Card.Title>Tip-Selection Performance</Card.Title>

                    <Line height={50} data={this.props.nodeStore.tipSelSeries} options={lineChartOptions}/>
                    <Line height={30} data={this.props.nodeStore.tipSelCacheSeries} options={cacheLineChartOpts}/>
                </Card.Body>
            </Card>
        );
    }
}
