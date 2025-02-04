
import { IconContext } from "react-icons";
import { ControlButtonsArray } from "./ControlButtonsArray";
import { FilterControl } from "./FilterControl";
import { useEffect } from "react";
import { EventsOn } from "../../wailsjs/runtime/runtime"

export function TopBar() {
    return (
        <IconContext.Provider value={{ size: "2em" }}>
            <div className="flex flex-row top-0 border-b w-screen items-center justify-between">
                <ControlButtonsArray />
                <FilterControl />
            </div>
        </IconContext.Provider >
    );
}