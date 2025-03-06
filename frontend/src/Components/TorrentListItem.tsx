import { torrentclient } from "../../wailsjs/go/models";
import { bytesToUpperUnit, torrentStateToString } from "../utils/utils";
import { ProgressBar } from "./ProgressBar";

interface TorrentListItemProps {
    torrent: torrentclient.TorrentDTO
}

export default function TorrentListItem({ torrent }: TorrentListItemProps) {

    const torrentSize = (torrent.torrentData.files?.length > 0) ? torrent.torrentData.torrentSize : torrent.torrentData.fileLength

    let downloadSpeed = 0;
    let uploadSpeed = 0;
    for (let [_, value] of Object.entries(torrent.torrentStats.connectionDataStatuses)) {
        downloadSpeed += value.downloadSpeed
        uploadSpeed += value.uploadSpeed
    }

    let barColorClass: string = "bg-green-600";
    let progress: number = 0;
    switch (torrent.torrentStats.torrentState) {
        case torrentclient.TorrentState.Running: {
            barColorClass = "bg-green-600";
            progress = (100 * torrent.torrentStats.piecesOnDiskCount / torrent.torrentData.pieceNumber);
            break;
        }
        case torrentclient.TorrentState.Rechecking: {
            barColorClass = "bg-blue-600";
            progress = (100 * torrent.torrentStats.recheckedPiecesCount / torrent.torrentData.pieceNumber);
            break;
        }
    }

    let ratio: string;
    if (torrent.torrentStats.downloadedBytes == 0) {
        if (torrent.torrentStats.uploadedBytes == 0) {
            ratio = "-";
        } else {
            ratio = (0).toFixed(2);
        }
    } else {
        ratio = (torrent.torrentStats.uploadedBytes / torrent.torrentStats.downloadedBytes).toFixed(2);
    }

    const typedTorrentSize = bytesToUpperUnit(torrentSize)
    const typedDownloadSpeed = bytesToUpperUnit(downloadSpeed)
    const typedUploadSpeed = bytesToUpperUnit(uploadSpeed)


    return (
        <tr>
            <td>
                {torrent.torrentData.name}
            </td>
            <td>{typedTorrentSize.dataAmount.toFixed(1)} {typedTorrentSize.unit}</td>
            <td>
                <ProgressBar barColorClass={barColorClass} progressPercent={progress}></ProgressBar>
            </td>
            <td>
                {torrentStateToString(torrent.torrentStats.torrentState)}
            </td>
            <td>
                {typedDownloadSpeed.dataAmount.toFixed(1)} {typedDownloadSpeed.unit}/s
            </td>
            <td>
                {typedUploadSpeed.dataAmount.toFixed(1)} {typedUploadSpeed.unit}/s
            </td>
            <td>
                {ratio}
            </td>
            <td>
                {torrent.torrentData.private ? "Yes" : "No"}
            </td>
        </tr >
    )
}