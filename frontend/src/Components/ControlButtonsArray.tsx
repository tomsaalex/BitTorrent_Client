import { FaLink } from "react-icons/fa"
import { FaPlusCircle } from "react-icons/fa";
import { FaPlay } from "react-icons/fa";
import { FaPause } from "react-icons/fa";
import { IoSettingsSharp } from "react-icons/io5";
import { IoTrashBin } from "react-icons/io5";

export function ControlButtonsArray() {
    return (
        <div className="inline-block divide-x">
            <div className="inline-block" id="listManagementButtons">
                <button className="p-2">
                    <FaLink />
                </button>
                <button className="p-2">
                    <FaPlusCircle />
                </button>
                <button className="p-2">
                    <IoTrashBin />
                </button>
            </div>
            <div className="inline-block" id="torrentControlButtons">
                <button className="p-2">
                    <FaPlay />
                </button>
                <button className="p-2">
                    <FaPause />
                </button>
            </div>
            <button className="p-2">
                <IoSettingsSharp />
            </button>
        </div>
    );
}