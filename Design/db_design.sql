-- Users Table
CREATE TABLE Users (
    UserID SERIAL PRIMARY KEY,
    username VARCHAR(255),
    password VARCHAR(255)
);

-- Venue Table
CREATE TABLE Venue (
    VenueID SERIAL PRIMARY KEY,
    VenueName VARCHAR(255),
    VenueLocation VARCHAR(255)
);

-- Hall Table
CREATE TABLE Hall (
    HallID SERIAL PRIMARY KEY,
    VenueID INTEGER REFERENCES Venue(VenueID),
    Capacity INTEGER,
);

-- Seat Table
CREATE TABLE Seat (
    SeatID VARCHAR(255) PRIMARY KEY,
    HallID INTEGER REFERENCES Hall(HallID),
    VenueID INTEGER REFERENCES Venue(VenueID),
    Price FLOAT,
    Category VARCHAR(255)
);

-- Show Table
CREATE TABLE Show (
    ShowID SERIAL PRIMARY KEY,
    ShowName VARCHAR(255),
    VenueID INTEGER REFERENCES Venue(VenueID),
    HallID INTEGER REFERENCES Hall(HallID),
    Time_start TIMESTAMP,
    Time_end TIMESTAMP,
    totalcapacity INTEGER,
    currentusage INTEGER
);

-- Reservation Table
CREATE TABLE Reservation (
    ReservationID SERIAL PRIMARY KEY,
    SeatReservationID VARCHAR(255),
    last_claim TIMESTAMP,
    ClaimedbyID INTEGER REFERENCES Users(UserID),
    BookedbyID INTEGER REFERENCES Users(UserID),
    Booked BOOLEAN,
    Booking_confirmID VARCHAR(255)
);



