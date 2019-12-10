import * as React from 'react';
import Card from "react-bootstrap/Card";
import {Line} from "react-chartjs-2";
import {inject, observer} from "mobx-react";
import NodeStore from "app/stores/NodeStore";
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
    },
}, defaultChartOptions);

@inject("nodeStore")
@observer
export default class RequestQueueChart extends React.Component<Props, any> {
    render() {
        return (
            <Card>
                <Card.Body>
                    <Card.Title>Request Queue</Card.Title>
                    <Line height={40} data={this.props.nodeStore.reqQSizeSeries} options={lineChartOptions}/>
                </Card.Body>
            </Card>
        );
    }
}
