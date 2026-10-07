import React, {useState} from "react";
import DatePicker from "react-datepicker";
import 'react-datepicker/dist/react-datepicker.css'

export const DatePick = (props) => {
    const [date, setDate] = useState(null);

    return(
        <div>
            <DatePicker 
                showIcon={true}
                dateFormat="dd/MM/yyyy"
                selected = {date}
                maxDate={new Date(new Date().getFullYear(), new Date().getMonth(), new Date().getDate())}
                onChange={(date) => {
                    setDate(date)
                    props.selectDate(date)
                }}
                icon="fa fa-calendar"
                placeholderText = {props.text}
            />
        </div>
    )
}
