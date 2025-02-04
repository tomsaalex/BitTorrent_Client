import wailsapp__runtime from "@wailsapp/runtime";
import { FaLink } from "react-icons/fa"
import { FaPlusCircle } from "react-icons/fa";
import { FaPlay } from "react-icons/fa";
import { FaPause } from "react-icons/fa";
import { IoSettingsSharp } from "react-icons/io5";
import { IoTrashBin } from "react-icons/io5";
import { EventsEmit } from "../../wailsjs/runtime/runtime";

export function ControlButtonsArray() {

    function triggerFileSelection() {
        EventsEmit("selectFile")
    }

    return (
        <div className="inline-block divide-x">
            <div className="inline-block" id="listManagementButtons">
                <button className="p-2">
                    <FaLink className="fill-green-600" />
                </button>
                <button className="p-2" onClick={() => { triggerFileSelection() }}>
                    <FaPlusCircle className="fill-green-600" />
                </button>
                <button className="p-2">
                    <IoTrashBin className="fill-red-600" />
                </button>
            </div>
            <div className="inline-block" id="torrentControlButtons">
                <button className="p-2">
                    <FaPlay className="fill-blue-600" />
                </button>
                <button className="p-2">
                    <FaPause className="fill-yellow-600" />
                </button>
            </div>
            <button className="p-2">
                <IoSettingsSharp className="fill-green-600" />
            </button>
        </div>
    );
}