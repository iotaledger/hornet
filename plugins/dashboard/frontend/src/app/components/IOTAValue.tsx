import * as React from 'react';
import ExplorerStore from "app/stores/ExplorerStore";
import {inject, observer} from "mobx-react";
import {OverlayTrigger, Tooltip} from "react-bootstrap";

type Props = {
    children: React.ReactNode;
    explorerStore?: ExplorerStore;
    showSign?: boolean;
};

@inject("explorerStore")
@observer
export class IOTAValue extends React.Component<Props, any> {

    render() {

        let num = this.props.children as number;
        let amount = Math.abs(num);

        let sign = (num < 0 ? "-" : (this.props.showSign ? "+" : ""));
        let unit = "";
        let value = amount;
        let fractions = 0;

        if (amount >= 1_000_000_000_000_000) {
            unit = 'P';
            value = (amount / 1_000_000_000_000_000)
            fractions = 2;
        } else if (amount >= 1_000_000_000_000) {
            unit = 'T';
            value = (amount / 1_000_000_000_000)
            fractions = 2;
        } else if (amount >= 1_000_000_000) {
            unit = 'G';
            value = (amount / 1_000_000_000)
            fractions = 2;
        } else if (amount >= 1_000_000) {
            unit = 'M';
            value = (amount / 1_000_000)
            fractions = 2;
        } else if (amount >= 1_000) {
            unit = 'K';
            value = (amount / 1_000)
            fractions = 1;
        }

        let formatted = `${sign}${value.toFixed(fractions)} ${unit}i`
        let unformatted = `${sign}${amount.toFixed(0)} i`

        let display = (this.props.explorerStore.shortenedValues ? formatted : unformatted)
        let tooltip = (this.props.explorerStore.shortenedValues ? unformatted : formatted)

        return (
            <OverlayTrigger
                placement="bottom"
                delay={{show: 150, hide: 150}}
                overlay={
                    <Tooltip id={`tooltip-value`}>
                        {tooltip}
                    </Tooltip>
                }
            >
                <div style={{display: "inline-block"}} onClick={() => this.props.explorerStore.toggleValueFormat()}>
                    {display}
                </div>
            </OverlayTrigger>
        );
    }
}