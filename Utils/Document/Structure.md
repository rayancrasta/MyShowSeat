
### Seat Table
We have venueid (int), hallid(int) note: one venue has many halls

seatid : is combination of venueid-hallid-seatno.

### Reservation Table

We have reservationid (autoincrement index)
Seatreservationid : "SH_" + reservationform.ShowID + "_ST_" + reservationform.SeatID //logic can be made more complex

