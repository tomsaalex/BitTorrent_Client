import { torrentclient } from "../../wailsjs/go/models";
import { TorrentsLoadingState } from "../slices/torrentSlice";

type TypedSize = {
    dataAmount: number;
    unit: string;
}

export function bytesToUpperUnit(byteCount: number) {
    let unitChanges = 0;

    while (byteCount >= 1024 && unitChanges < 8) {
        byteCount /= 1024;
        unitChanges++
    }

    let unit: string;
    switch (unitChanges) {
        case 0: unit = 'B'; break;
        case 1: unit = 'KiB'; break;
        case 2: unit = 'MiB'; break;
        case 3: unit = 'GiB'; break;
        case 4: unit = 'TiB'; break;
        case 5: unit = 'PiB'; break;
        case 6: unit = 'EiB'; break;
        case 7: unit = 'ZiB'; break;
        case 8: unit = 'YiB'; break;
        default: unit = "";
    }

    let convertedSize: TypedSize = { dataAmount: byteCount, unit }
    return convertedSize;
}

export function torrentStateToString(torrentState: torrentclient.TorrentState): string {
    switch (torrentState) {
        case torrentclient.TorrentState.Running: return "Running"
        case torrentclient.TorrentState.Paused: return "Paused"
        case torrentclient.TorrentState.Rechecking: return "Rechecking"
    }
}